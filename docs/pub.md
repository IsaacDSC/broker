# Package Pub - Sistema de Publishers/Produtores

## Visão Geral

O package `pub` é responsável pelo sistema de publishers (produtores) do broker. Ele fornece uma interface simples e direta para publicação de mensagens no sistema, abstraindo a complexidade da interação com a Queue e garantindo que as mensagens sejam armazenadas corretamente para processamento posterior pelos subscribers.

## Responsabilidades Principais

- **Publicação de Mensagens**: Interface principal para envio de mensagens ao sistema
- **Abstração da Queue**: Esconde complexidade da implementação de armazenamento
- **Validação de Dados**: Garante que mensagens sejam publicadas corretamente
- **Error Handling**: Trata erros de publicação de forma consistente
- **Performance**: Operações otimizadas para alta throughput
- **Simplicidade**: API limpa e fácil de usar

## Arquitetura

### Estrutura Principal

```go
type Broker struct {
    *broker.Client    // Cliente broker para acesso aos componentes
}
```

### Componentes Integrados

- **Client**: Acesso direto aos componentes Queue e Balancer
- **Queue**: Interface para armazenamento de mensagens
- **EventBrokerKey**: Sistema de namespacing para eventos

## Funções Principais

### `NewBroker(client *broker.Client) *Broker`

**Responsabilidade**: Cria nova instância de publisher
**Parâmetros**:

- `client`: Cliente broker configurado com Queue e Balancer
  **Retorno**: Instância configurada do Broker
  **Comportamento**:
- Cria wrapper em torno do cliente broker
- Fornece interface especializada para publicação
- Reutiliza conexões e configurações existentes

**Características**:

- ✅ Lightweight (apenas wrapper)
- ✅ Reutiliza recursos do cliente
- ✅ Thread-safe por herança
- ✅ Sem estado adicional

**Exemplo**:

```go
client, _ := broker.NewRedisClient("app", "localhost:6379")
publisher := pub.NewBroker(client)
```

### `Publish(event string, payload any) error`

**Responsabilidade**: Publica mensagem no sistema de filas
**Parâmetros**:

- `event`: Nome/tipo do evento (ex: "user.created", "order.placed")
- `payload`: Dados da mensagem (qualquer tipo serializável)
  **Retorno**: Error se falhar na publicação
  **Comportamento**:
- Delega para client.Publish() que usa Queue.Store()
- Gera automaticamente ID único para a mensagem
- Aplica namespacing via EventBrokerKey()
- Operação atômica e thread-safe

**Fluxo de Execução**:

```
Publisher.Publish(event, payload)
    ↓
Client.Publish(event, payload)
    ↓
Queue.Store(event, payload)
    ↓
Message criada com UUID único
    ↓
Armazenada na fila para processamento
```

**Exemplo Básico**:

```go
publisher := pub.NewBroker(client)

err := publisher.Publish("user.created", map[string]any{
    "id":    123,
    "name":  "João Silva",
    "email": "joao@example.com",
})

if err != nil {
    log.Printf("Erro ao publicar: %v", err)
}
```

## Tipos de Payload Suportados

### Tipos Primitivos

```go
// String
publisher.Publish("message.sent", "Hello World")

// Números
publisher.Publish("counter.increment", 42)
publisher.Publish("price.updated", 99.99)

// Boolean
publisher.Publish("feature.enabled", true)
```

### Estruturas de Dados

```go
// Map
publisher.Publish("user.created", map[string]any{
    "id":       123,
    "name":     "João",
    "email":    "joao@example.com",
    "active":   true,
    "metadata": map[string]string{"source": "api"},
})

// Slice
publisher.Publish("batch.processed", []string{"item1", "item2", "item3"})

// Struct
type User struct {
    ID    int    `json:"id"`
    Name  string `json:"name"`
    Email string `json:"email"`
}

user := User{ID: 123, Name: "João", Email: "joao@example.com"}
publisher.Publish("user.created", user)
```

### Dados Complexos

```go
// JSON-like structures
publisher.Publish("order.placed", map[string]any{
    "order_id": "ORD-123",
    "customer": map[string]any{
        "id":   456,
        "name": "Maria",
    },
    "items": []map[string]any{
        {"product": "Laptop", "price": 1500.00, "qty": 1},
        {"product": "Mouse",  "price": 25.00,   "qty": 2},
    },
    "total":     1550.00,
    "timestamp": time.Now(),
})
```

## Padrões de Uso

### Publisher Simples

```go
func main() {
    client, err := broker.NewMemoryClient("my-app")
    if err != nil {
        log.Fatal(err)
    }
    defer client.Close()

    publisher := pub.NewBroker(client)

    // Publicar evento
    err = publisher.Publish("user.signup", map[string]any{
        "user_id": 123,
        "email":   "user@example.com",
        "source":  "web",
    })

    if err != nil {
        log.Printf("Erro: %v", err)
    }
}
```

### Batch Publishing

```go
func publishBatch(publisher *pub.Broker, events []Event) error {
    var errors []error

    for _, event := range events {
        if err := publisher.Publish(event.Type, event.Data); err != nil {
            errors = append(errors, fmt.Errorf("evento %s: %w", event.Type, err))
        }
    }

    if len(errors) > 0 {
        return fmt.Errorf("erros na publicação: %v", errors)
    }

    return nil
}

// Uso
events := []Event{
    {Type: "user.created", Data: userData1},
    {Type: "user.created", Data: userData2},
    {Type: "email.queued", Data: emailData},
}

publishBatch(publisher, events)
```

### Publisher com Retry

```go
func publishWithRetry(publisher *pub.Broker, event string, payload any, maxRetries int) error {
    for i := 0; i < maxRetries; i++ {
        if err := publisher.Publish(event, payload); err != nil {
            log.Printf("Tentativa %d falhou: %v", i+1, err)

            // Backoff exponencial
            backoff := time.Duration(i+1) * time.Second
            time.Sleep(backoff)
            continue
        }

        return nil // Sucesso
    }

    return fmt.Errorf("falha após %d tentativas", maxRetries)
}

// Uso
err := publishWithRetry(publisher, "payment.processed", paymentData, 3)
```

### Publisher com Validação

```go
type ValidatingPublisher struct {
    *pub.Broker
    validators map[string]func(any) error
}

func NewValidatingPublisher(client *broker.Client) *ValidatingPublisher {
    return &ValidatingPublisher{
        Broker:     pub.NewBroker(client),
        validators: make(map[string]func(any) error),
    }
}

func (vp *ValidatingPublisher) RegisterValidator(event string, validator func(any) error) {
    vp.validators[event] = validator
}

func (vp *ValidatingPublisher) Publish(event string, payload any) error {
    // Validar se há validator
    if validator, exists := vp.validators[event]; exists {
        if err := validator(payload); err != nil {
            return fmt.Errorf("validação falhou para %s: %w", event, err)
        }
    }

    // Publicar se válido
    return vp.Broker.Publish(event, payload)
}

// Uso
vp := NewValidatingPublisher(client)

vp.RegisterValidator("user.created", func(payload any) error {
    user, ok := payload.(map[string]any)
    if !ok {
        return fmt.Errorf("payload deve ser map[string]any")
    }

    if user["email"] == nil {
        return fmt.Errorf("email é obrigatório")
    }

    return nil
})

// Publicar com validação
err := vp.Publish("user.created", userData)
```

### Publisher Assíncrono

```go
type AsyncPublisher struct {
    *pub.Broker
    queue chan PublishRequest
    done  chan struct{}
}

type PublishRequest struct {
    Event   string
    Payload any
    Result  chan error
}

func NewAsyncPublisher(client *broker.Client, bufferSize int) *AsyncPublisher {
    ap := &AsyncPublisher{
        Broker: pub.NewBroker(client),
        queue:  make(chan PublishRequest, bufferSize),
        done:   make(chan struct{}),
    }

    go ap.worker()
    return ap
}

func (ap *AsyncPublisher) worker() {
    for {
        select {
        case req := <-ap.queue:
            err := ap.Broker.Publish(req.Event, req.Payload)
            req.Result <- err
        case <-ap.done:
            return
        }
    }
}

func (ap *AsyncPublisher) PublishAsync(event string, payload any) <-chan error {
    result := make(chan error, 1)

    select {
    case ap.queue <- PublishRequest{Event: event, Payload: payload, Result: result}:
        return result
    default:
        result <- fmt.Errorf("queue buffer full")
        return result
    }
}

func (ap *AsyncPublisher) Close() {
    close(ap.done)
}

// Uso
asyncPub := NewAsyncPublisher(client, 100)
defer asyncPub.Close()

errChan := asyncPub.PublishAsync("user.created", userData)
if err := <-errChan; err != nil {
    log.Printf("Erro: %v", err)
}
```

## Integração com Diferentes Backends

### Memory Backend

```go
client, _ := broker.NewMemoryClient("app")
publisher := pub.NewBroker(client)

// Características:
// ✅ Ultra-rápido (nanossegundos)
// ✅ Sem dependências externas
// ❌ Não persiste entre reinicializações
// ❌ Limitado a single-instance
```

### Redis Backend

```go
client, _ := broker.NewRedisClient("app", "localhost:6379")
publisher := pub.NewBroker(client)

// Características:
// ✅ Persistência garantida
// ✅ Distribuído entre instâncias
// ❌ Latência de rede (milissegundos)
// ❌ Dependência externa
```

### Hybrid Backend

```go
client, _ := broker.NewHybridClient("app", "localhost:6379")
publisher := pub.NewBroker(client)

// Características:
// ✅ Queue persistente (Redis)
// ✅ Balancer rápido (Memory)
// ⚠️ Configuração específica
```

## Performance e Otimizações

### Benchmarks Típicos

```go
// Memory Backend
BenchmarkPublishMemory-8    10000000    150 ns/op    48 B/op    2 allocs/op

// Redis Backend
BenchmarkPublishRedis-8     100000      15000 ns/op  200 B/op   5 allocs/op
```

### Otimizações para Alta Throughput

#### Batch Publishing

```go
func publishBatchOptimized(publisher *pub.Broker, events []Event) error {
    // Agrupar por tipo de evento
    eventGroups := make(map[string][]any)
    for _, event := range events {
        eventGroups[event.Type] = append(eventGroups[event.Type], event.Data)
    }

    // Publicar em grupos
    for eventType, payloads := range eventGroups {
        for _, payload := range payloads {
            if err := publisher.Publish(eventType, payload); err != nil {
                return err
            }
        }
    }

    return nil
}
```

#### Connection Pooling (Redis)

```go
// Configuração otimizada para Redis
cfg := config.NewRedisConfig("app", "redis:6379")
cfg.Queue.Redis.PoolSize = 50        // Pool maior
cfg.Queue.Redis.MinIdleConns = 10    // Mais conexões idle
cfg.Queue.Redis.MaxRetries = 1       // Fail fast

client, _ := broker.NewClientWithConfig(cfg)
```

#### Payload Optimization

```go
// ❌ Evitar payloads muito grandes
publisher.Publish("event", hugeBinaryData) // Lento

// ✅ Usar referências ou URLs
publisher.Publish("event", map[string]any{
    "file_url": "https://storage.com/file123",
    "metadata": smallMetadata,
})
```

## Error Handling

### Tipos de Erros

#### Configuração/Conexão

```go
err := publisher.Publish("event", data)
if err != nil {
    // Possíveis erros:
    // - Redis connection failed
    // - Queue storage full
    // - Serialization failed
    log.Printf("Erro de infraestrutura: %v", err)
}
```

#### Payload/Serialização

```go
// Payloads que não podem ser serializados (Redis)
type BadStruct struct {
    Channel chan int // Não serializável
}

err := publisher.Publish("event", BadStruct{})
// Erro: json: unsupported type: chan int
```

### Estratégias de Recovery

#### Circuit Breaker Pattern

```go
type CircuitBreakerPublisher struct {
    *pub.Broker
    breaker *CircuitBreaker
}

func (cbp *CircuitBreakerPublisher) Publish(event string, payload any) error {
    return cbp.breaker.Call(func() error {
        return cbp.Broker.Publish(event, payload)
    })
}
```

#### Dead Letter Queue

```go
type DLQPublisher struct {
    *pub.Broker
    dlq *pub.Broker
}

func (dlq *DLQPublisher) Publish(event string, payload any) error {
    if err := dlq.Broker.Publish(event, payload); err != nil {
        // Tentar DLQ
        dlqEvent := fmt.Sprintf("dlq.%s", event)
        dlqPayload := map[string]any{
            "original_event": event,
            "original_payload": payload,
            "error": err.Error(),
            "timestamp": time.Now(),
        }

        return dlq.dlq.Publish(dlqEvent, dlqPayload)
    }

    return nil
}
```

## Monitoramento e Métricas

### Métricas Importantes

```go
type PublisherMetrics struct {
    MessagesPublished int64
    PublishErrors     int64
    AverageLatency    time.Duration
    ThroughputPerSec  float64
}
```

### Instrumentação

```go
type InstrumentedPublisher struct {
    *pub.Broker
    metrics *PublisherMetrics
}

func (ip *InstrumentedPublisher) Publish(event string, payload any) error {
    start := time.Now()

    err := ip.Broker.Publish(event, payload)

    // Métricas
    latency := time.Since(start)
    if err != nil {
        atomic.AddInt64(&ip.metrics.PublishErrors, 1)
    } else {
        atomic.AddInt64(&ip.metrics.MessagesPublished, 1)
    }

    // Atualizar latência média (simplificado)
    // Implementação real usaria sliding window

    return err
}
```

### Health Checks

```go
func (p *pub.Broker) HealthCheck() error {
    // Publicar mensagem de teste
    testEvent := "health.check"
    testPayload := map[string]any{
        "timestamp": time.Now(),
        "test":      true,
    }

    return p.Publish(testEvent, testPayload)
}
```

## Integração com Frameworks

### HTTP API

```go
func publishHandler(publisher *pub.Broker) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        var req struct {
            Event   string `json:"event"`
            Payload any    `json:"payload"`
        }

        if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
            http.Error(w, "Invalid JSON", http.StatusBadRequest)
            return
        }


        if err := publisher.Publish(req.Event, req.Payload); err != nil {
            http.Error(w, err.Error(), http.StatusInternalServerError)
            return
        }

        w.WriteHeader(http.StatusAccepted)
    }
}

// Uso
http.HandleFunc("/publish", publishHandler(publisher))
```

### gRPC Service

```go
type PublisherService struct {
    publisher *pub.Broker
}

func (ps *PublisherService) Publish(ctx context.Context, req *pb.PublishRequest) (*pb.PublishResponse, error) {
    // Converter protobuf para any
    var payload any
    if err := json.Unmarshal(req.PayloadJson, &payload); err != nil {
        return nil, status.Errorf(codes.InvalidArgument, "invalid payload: %v", err)
    }

    if err := ps.publisher.Publish(req.Event, payload); err != nil {
        return nil, status.Errorf(codes.Internal, "publish failed: %v", err)
    }

    return &pb.PublishResponse{Success: true}, nil
}
```

### Middleware para Logging

```go
func LoggingMiddleware(publisher *pub.Broker) *pub.Broker {
    return &pub.Broker{
        Client: &LoggingClient{
            Client: publisher.Client,
        },
    }
}

type LoggingClient struct {
    *broker.Client
}

func (lc *LoggingClient) Publish(event string, payload any) error {
    log.Printf("Publishing event: %s, payload: %v", event, payload)

    start := time.Now()
    err := lc.Client.Publish(event, payload)
    duration := time.Since(start)

    if err != nil {
        log.Printf("Publish failed for %s: %v (took %v)", event, err, duration)
    } else {
        log.Printf("Published %s successfully (took %v)", event, duration)
    }

    return err
}
```

## Testing

### Unit Tests

```go
func TestPublisher(t *testing.T) {
    client, _ := broker.NewMemoryClient("test")
    defer client.Close()

    publisher := pub.NewBroker(client)

    // Test básico
    err := publisher.Publish("test.event", "test data")
    assert.NoError(t, err)

    // Verificar se foi armazenado
    length, _ := client.Queue.Len()
    assert.Equal(t, 1, length)
}
```

### Integration Tests

```go
func TestPublisherRedis(t *testing.T) {
    // Requer Redis rodando
    client, err := broker.NewRedisClient("test", "localhost:6379")
    require.NoError(t, err)
    defer client.Close()

    publisher := pub.NewBroker(client)

    err = publisher.Publish("integration.test", map[string]any{
        "test": true,
        "data": 123,
    })
    assert.NoError(t, err)
}
```

### Benchmark Tests

```go
func BenchmarkPublishMemory(b *testing.B) {
    client, _ := broker.NewMemoryClient("bench")
    defer client.Close()

    publisher := pub.NewBroker(client)
    payload := map[string]any{"test": "data"}

    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        publisher.Publish("bench.event", payload)
    }
}
```

## Melhores Práticas

### 1. Nomeação de Eventos

```go
// ✅ Use convenção consistente
publisher.Publish("user.created", userData)
publisher.Publish("user.updated", userData)
publisher.Publish("user.deleted", userData)

// ❌ Evitar nomes inconsistentes
publisher.Publish("newUser", userData)
publisher.Publish("UpdateUser", userData)
publisher.Publish("delete_user", userData)
```

### 2. Estrutura de Payload

```go
// ✅ Estrutura consistente
type EventPayload struct {
    ID        string    `json:"id"`
    Timestamp time.Time `json:"timestamp"`
    Data      any       `json:"data"`
    Metadata  any       `json:"metadata,omitempty"`
}

publisher.Publish("user.created", EventPayload{
    ID:        uuid.New().String(),
    Timestamp: time.Now(),
    Data:      userData,
    Metadata:  map[string]string{"source": "api"},
})
```

### 3. Error Handling Robusto

```go
// ✅ Sempre tratar erros
if err := publisher.Publish("event", data); err != nil {
    log.Printf("Falha na publicação: %v", err)
    // Implementar retry, alertas, métricas, etc.
}
```

### 4. Payloads Eficientes

```go
// ✅ Payloads otimizados
publisher.Publish("user.created", map[string]any{
    "id":    user.ID,
    "email": user.Email,
    // Apenas campos necessários
})

// ❌ Evitar objetos muito grandes
publisher.Publish("user.created", entireUserObjectWithAllFields)
```

### 5. Versionamento de Eventos

```go
// ✅ Versionar eventos para compatibilidade
publisher.Publish("user.created.v2", userDataV2)

// Ou usar metadata
publisher.Publish("user.created", map[string]any{
    "version": "2.0",
    "data":    userData,
})
```

## Troubleshooting

### Problemas Comuns

#### Publicação Lenta

- Verificar latência da conexão Redis
- Monitorar tamanho dos payloads
- Analisar pool de conexões

#### Mensagens Perdidas

- Verificar se erros estão sendo tratados
- Confirmar persistência do backend
- Analisar logs de falhas

#### Serialização Falha

- Validar tipos de dados no payload
- Evitar canais, funções, ponteiros complexos
- Testar serialização JSON se usar Redis

### Debug Tools

```go
// Debug publisher
func debugPublisher(p *pub.Broker) {
    // Health check
    if err := p.HealthCheck(); err != nil {
        log.Printf("Publisher unhealthy: %v", err)
    }

    // Queue stats
    stats, _ := p.Client.GetStats()
    log.Printf("Queue length: %v", stats["queue_length"])

    // Config info
    p.Client.PrintConfig()
}
```

## Extensibilidade

### Custom Publisher

```go
type CustomPublisher struct {
    *pub.Broker
    enricher func(string, any) any
}

func (cp *CustomPublisher) Publish(event string, payload any) error {
    // Enriquecer payload
    enrichedPayload := cp.enricher(event, payload)

    return cp.Broker.Publish(event, enrichedPayload)
}
```

### Plugin System

```go
type PublisherPlugin interface {
    BeforePublish(event string, payload any) (any, error)
    AfterPublish(event string, payload any, err error)
}

type PluginPublisher struct {
    *pub.Broker
    plugins []PublisherPlugin
}

func (pp *PluginPublisher) Publish(event string, payload any) error {
    // Before plugins
    for _, plugin := range pp.plugins {
        var err error
        payload, err = plugin.BeforePublish(event, payload)
        if err != nil {
            return err
        }
    }

    // Publish
    err := pp.Broker.Publish(event, payload)

    // After plugins
    for _, plugin := range pp.plugins {
        plugin.AfterPublish(event, payload, err)
    }

    return err
}
```
