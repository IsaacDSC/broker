# Package Sub - Sistema de Subscribers/Consumidores

## Visão Geral

O package `sub` é responsável pelo sistema de subscribers (consumidores) do broker. Ele gerencia o registro de handlers para eventos específicos, coordena com o sistema de balanceamento de carga para distribuição de mensagens, e garante que cada mensagem seja processada exatamente uma vez por apenas um consumidor ativo.

## Responsabilidades Principais

- **Registro de Handlers**: Permite registrar funções para processar eventos específicos
- **Listening de Mensagens**: Monitora continuamente a fila em busca de novas mensagens
- **Coordenação com Balancer**: Integra-se com o sistema de balanceamento para distribuição
- **Controle de Concorrência**: Gerencia processamento paralelo com semáforos
- **Heartbeat Management**: Mantém presença ativa no sistema de balanceamento
- **Graceful Shutdown**: Permite encerramento seguro com cleanup de recursos

## Arquitetura

### Estrutura Principal

```go
type Subscribe struct {
    *broker.Client              // Cliente broker para acesso aos componentes
    subID        uuid.UUID      // Identificador único deste subscriber
    subscribers  map[string]Subscriber  // Mapa de handlers por evento
    semaphore    chan struct{}  // Controle de concorrência
    stopCh       chan struct{}  // Canal para sinalizar parada
    processingWg sync.WaitGroup // Wait group para aguardar processamento
}
```

### Componentes Integrados

#### Subscriber Handler
```go
type Subscriber struct {
    Event   string              // Nome do evento
    Handler SubscriberHandler   // Função handler
}

type SubscriberHandler func(subctx Ctx) error
```

#### Context
```go
type Ctx struct {
    ctx     context.Context     // Context Go padrão
    payload any                // Dados da mensagem
}

func (ctx Ctx) GetPayload() any {
    return ctx.payload
}
```

## Constantes e Configurações

### Controle de Concorrência
```go
const (
    MaxConcurrency = 10  // Máximo de mensagens processadas simultaneamente
)
```

## Funções Principais

### `NewSubscribe(conn *broker.Client) *Subscribe`

**Responsabilidade**: Cria nova instância de subscriber
**Parâmetros**:
- `conn`: Cliente broker configurado
**Retorno**: Instância configurada do Subscribe
**Comportamento**:
- Gera UUID único para o subscriber
- Inicializa estruturas de controle (semáforo, canais, maps)
- Registra automaticamente no balancer do cliente
- Configura controle de concorrência com MaxConcurrency

**Inicialização Interna**:
```go
sub := &Subscribe{
    Client:      conn,
    subID:       uuid.New(),
    subscribers: make(map[string]Subscriber),
    semaphore:   make(chan struct{}, MaxConcurrency),
    stopCh:      make(chan struct{}),
}

// Auto-registro no balancer
conn.Balancer.Subscribe(sub.subID)
return sub
```

**Exemplo**:
```go
client, _ := broker.NewRedisClient("app", "localhost:6379")
subscriber := sub.NewSubscribe(client)
```

### `WithSubscriber(eventName string, handler SubscriberHandler) *Subscribe`

**Responsabilidade**: Registra handler para um evento específico
**Parâmetros**:
- `eventName`: Nome do evento a processar (ex: "user.created")
- `handler`: Função que processará mensagens deste evento
**Retorno**: Self para method chaining
**Comportamento**:
- Converte eventName para chave broker usando EventBrokerKey()
- Armazena handler no mapa interno de subscribers
- Permite fluent interface para registrar múltiplos eventos

**Chaining Pattern**:
```go
subscriber := sub.NewSubscribe(client).
    WithSubscriber("user.created", handleUserCreated).
    WithSubscriber("user.updated", handleUserUpdated).
    WithSubscriber("order.placed", handleOrderPlaced)
```

**Handler Signature**:
```go
func handleUserCreated(ctx sub.Ctx) error {
    userData := ctx.GetPayload()
    log.Printf("Processando usuário: %v", userData)

    // Lógica de processamento
    return nil // ou erro se falhar
}
```

### `Listener() error`

**Responsabilidade**: Inicia loop principal de processamento de mensagens
**Retorno**: Error se falhar na inicialização ou durante processamento
**Comportamento**:
- Valida se há subscribers registrados
- Inicia loop infinito de monitoramento
- Para gracefully quando recebe sinal de stop
- Aguarda processamento de mensagens em andamento antes de encerrar

**Algoritmo Principal**:
```go
func (b *Subscribe) Listener() error {
    // Validações iniciais
    if len(b.subscribers) == 0 {
        return errors.New("no subscribers")
    }

    for {
        select {
        case <-b.stopCh:
            // Cleanup e encerramento graceful
            b.Balancer.Unsubscribe(b.subID)
            b.processingWg.Wait()
            return nil
        default:
            // Processar mensagens disponíveis
            b.processAvailableMessages()
        }
    }
}
```

### `processAvailableMessages()`

**Responsabilidade**: Processa mensagens disponíveis na fila (função interna)
**Comportamento**:
- Itera sobre todos os tipos de eventos registrados
- Para cada tipo, busca mensagens não-claimed
- Verifica com balancer se deve processar cada mensagem
- Claims mensagem atomicamente se autorizado
- Inicia processamento assíncrono se conseguir slot no semáforo

**Algoritmo Detalhado**:
```go
func (b *Subscribe) processAvailableMessages() {
    // Para cada evento registrado
    for eventKey := range b.subscribers {
        eventName := b.extractEventName(eventKey)

        // Buscar mensagens não-claimed
        messages, err := b.Queue.GetUnclaimedMessagesByKey(eventName)
        if err != nil {
            continue
        }

        for _, msg := range messages {
            // Verificar com balancer se deve processar
            shouldProcess, err := b.Balancer.ClaimMessage(b.subID, msg.ID)
            if err != nil || !shouldProcess {
                continue
            }

            // Verificar se já foi processada
            if b.Queue.IsProcessed(msg.ID) {
                continue
            }

            // Claim atômico da mensagem
            if !b.Queue.ClaimMessage(msg.ID) {
                continue
            }

            // Tentar adquirir slot de processamento
            select {
            case b.semaphore <- struct{}{}:
                b.processingWg.Add(1)
                go b.processMessage(msg.ID, eventName, msg.Value)
            default:
                // Se não conseguir slot, continua no próximo ciclo
                continue
            }
        }
    }
}
```

### `processMessage(messageID, eventName string, msg any)`

**Responsabilidade**: Processa uma mensagem específica (goroutine)
**Parâmetros**:
- `messageID`: ID único da mensagem
- `eventName`: Tipo do evento
- `msg`: Payload da mensagem
**Comportamento**:
- Executa handler registrado para o evento
- Gerencia recursos (semáforo, wait group)
- Marca mensagem como processada
- Atualiza heartbeat no balancer
- Trata erros de forma robusta

**Lifecycle Completo**:
```go
func (b *Subscribe) processMessage(messageID, eventName string, msg any) {
    defer func() {
        <-b.semaphore                    // Libera slot do semáforo
        b.processingWg.Done()            // Sinaliza conclusão
        b.Queue.MarkAsProcessed(messageID)  // Marca como processada
        b.Balancer.UpdateHeartbeat(b.subID) // Atualiza heartbeat
    }()

    // Buscar handler
    subscriber, ok := b.subscribers[b.EventBrokerKey(eventName)]
    if !ok {
        log.Println("[*] Not subscriber on event: ", eventName)
        return
    }

    // Executar handler
    ctx := context.Background()
    if err := subscriber.Handler(Ctx{ctx, msg}); err != nil {
        log.Println("[*] Error handling event: ", eventName, err)
    }
}
```

### `Stop()`

**Responsabilidade**: Para o subscriber gracefully
**Comportamento**:
- Fecha canal stopCh para sinalizar encerramento
- Faz com que Listener() encerre o loop principal
- Cleanup é feito automaticamente pelo Listener()

**Uso**:
```go
// Em signal handler ou shutdown hook
go func() {
    <-sigterm
    subscriber.Stop()
}()
```

## Métodos Auxiliares

### `extractEventName(eventKey string) string`

**Responsabilidade**: Extrai nome do evento da chave broker
**Parâmetros**:
- `eventKey`: Chave no formato "gqueue:app-name:event"
**Retorno**: Nome limpo do evento
**Comportamento**:
- Parse reverso da chave gerada por EventBrokerKey()
- Extrai apenas a parte do evento
- Usado internamente para processar mensagens

### `GetSubscriberID() uuid.UUID`

**Responsabilidade**: Retorna ID único deste subscriber
**Retorno**: UUID do subscriber
**Uso**: Debug, logging, identificação

### `GetSubscriberInfo() (interface{}, error)`

**Responsabilidade**: Retorna informações do subscriber via balancer
**Retorno**: Estrutura SubscriberInfo ou erro
**Uso**: Monitoramento, debug, métricas

### `SetWeight(weight int) error`

**Responsabilidade**: Define peso deste subscriber para balanceamento
**Parâmetros**:
- `weight`: Peso relativo (maior = mais mensagens)
**Retorno**: Error se falhar
**Uso**: Balanceamento weighted, ajuste de capacidade

### `UpdateHeartbeat() error`

**Responsabilidade**: Força atualização do heartbeat
**Retorno**: Error se falhar
**Uso**: Manter presença ativa, debugging

## Padrões de Uso

### Subscriber Simples

```go
func main() {
    client, _ := broker.NewMemoryClient("my-app")
    defer client.Close()

    subscriber := sub.NewSubscribe(client).
        WithSubscriber("user.created", func(ctx sub.Ctx) error {
            user := ctx.GetPayload()
            fmt.Printf("Novo usuário: %v\n", user)
            return nil
        })

    log.Println("Iniciando listener...")
    if err := subscriber.Listener(); err != nil {
        log.Fatal(err)
    }
}
```

### Múltiplos Eventos

```go
subscriber := sub.NewSubscribe(client).
    WithSubscriber("user.created", handleUserCreated).
    WithSubscriber("user.updated", handleUserUpdated).
    WithSubscriber("user.deleted", handleUserDeleted).
    WithSubscriber("order.placed", handleOrderPlaced).
    WithSubscriber("payment.processed", handlePaymentProcessed)

// Handlers
func handleUserCreated(ctx sub.Ctx) error {
    userData := ctx.GetPayload().(map[string]interface{})

    // Validação
    if userData["email"] == nil {
        return fmt.Errorf("email obrigatório")
    }

    // Processamento
    return userService.Create(userData)
}

func handleOrderPlaced(ctx sub.Ctx) error {
    orderData := ctx.GetPayload().(map[string]interface{})

    // Processamento assíncrono
    go func() {
        inventoryService.Reserve(orderData["items"])
        emailService.SendConfirmation(orderData["user_email"])
    }()

    return nil
}
```

### Processamento com Error Handling

```go
func handleUserCreated(ctx sub.Ctx) error {
    userData := ctx.GetPayload()

    // Log estruturado
    logger := log.With("event", "user.created", "data", userData)
    logger.Info("Processando criação de usuário")

    // Retry logic
    maxRetries := 3
    for i := 0; i < maxRetries; i++ {
        if err := userService.Create(userData); err != nil {
            logger.Warn("Tentativa falhou", "retry", i+1, "error", err)
            time.Sleep(time.Duration(i+1) * time.Second)
            continue
        }

        logger.Info("Usuário criado com sucesso")
        return nil
    }

    // Dead letter queue ou alerta
    return fmt.Errorf("falha após %d tentativas", maxRetries)
}
```

### Subscriber com Graceful Shutdown

```go
func runSubscriber() {
    client, _ := broker.NewRedisClient("app", "redis:6379")
    defer client.Close()

    subscriber := sub.NewSubscribe(client).
        WithSubscriber("events", handleEvent)

    // Canal para sinais do OS
    sigChan := make(chan os.Signal, 1)
    signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

    // Goroutine para shutdown
    go func() {
        <-sigChan
        log.Println("Recebido sinal de shutdown...")
        subscriber.Stop()
    }()

    log.Println("Iniciando subscriber...")
    if err := subscriber.Listener(); err != nil {
        log.Printf("Subscriber encerrado: %v", err)
    }
}
```

### Subscriber com Monitoramento

```go
func createMonitoredSubscriber(client *broker.Client) *sub.Subscribe {
    subscriber := sub.NewSubscriber(client)

    // Configurar peso baseado na capacidade
    subscriber.SetWeight(getInstanceCapacity())

    // Monitoramento periódico
    go func() {
        ticker := time.NewTicker(30 * time.Second)
        defer ticker.Stop()

        for range ticker.C {
            info, err := subscriber.GetSubscriberInfo()
            if err != nil {
                log.Printf("Erro obtendo info: %v", err)
                continue
            }

            // Enviar métricas para sistema de monitoramento
            metrics.Gauge("subscriber.process_count").Set(info.ProcessCount)
            metrics.Gauge("subscriber.weight").Set(info.Weight)

            // Heartbeat manual se necessário
            subscriber.UpdateHeartbeat()
        }
    }()

    return subscriber
}
```

## Controle de Concorrência

### Semáforo de Processamento

O subscriber usa um semáforo para controlar quantas mensagens podem ser processadas simultaneamente:

```go
const MaxConcurrency = 10
semaphore := make(chan struct{}, MaxConcurrency)

// Adquirir slot
select {
case semaphore <- struct{}{}:
    // Processar mensagem
    go processMessage()
default:
    // Sem slots disponíveis, tentar depois
}
```

### WaitGroup para Shutdown

```go
var processingWg sync.WaitGroup

// Antes de processar
processingWg.Add(1)
go func() {
    defer processingWg.Done()
    // Processar mensagem
}()

// No shutdown
processingWg.Wait() // Aguarda todas as goroutines terminarem
```

### Thread Safety

- **subscribers map**: Protegido por acesso single-threaded (só modificado na criação)
- **semaphore channel**: Thread-safe por natureza
- **Queue operations**: Thread-safe via implementação da interface
- **Balancer operations**: Thread-safe via implementação da interface

## Integração com Balancer

### Registro Automático

```go
// No NewSubscribe()
conn.Balancer.Subscribe(sub.subID)
```

### Claim de Mensagens

```go
// Verificar se deve processar
shouldProcess, err := b.Balancer.ClaimMessage(b.subID, msg.ID)
if err != nil || !shouldProcess {
    continue // Outro subscriber processará
}
```

### Heartbeat Management

```go
// Após processar mensagem
b.Balancer.UpdateHeartbeat(b.subID)
```

### Cleanup no Shutdown

```go
// No Listener() quando recebe stop signal
b.Balancer.Unsubscribe(b.subID)
```

## Error Handling

### Tipos de Erros

#### Configuração
```go
if len(b.subscribers) == 0 {
    return errors.New("no subscribers")
}
```

#### Processamento
```go
if err := subscriber.Handler(Ctx{ctx, msg}); err != nil {
    log.Println("[*] Error handling event: ", eventName, err)
    // Mensagem é marcada como processada mesmo com erro
    // Implementar dead letter queue se necessário
}
```

#### Balancer/Queue
```go
messages, err := b.Queue.GetUnclaimedMessagesByKey(eventName)
if err != nil {
    log.Printf("Failed to get messages for event %s: %v", eventName, err)
    continue // Tentar próximo evento
}
```

### Estratégias de Recovery

#### Retry no Handler
```go
func handleWithRetry(ctx sub.Ctx) error {
    maxRetries := 3
    for i := 0; i < maxRetries; i++ {
        if err := processPayload(ctx.GetPayload()); err != nil {
            log.Printf("Retry %d failed: %v", i+1, err)
            time.Sleep(time.Duration(i+1) * time.Second)
            continue
        }
        return nil
    }
    return fmt.Errorf("failed after %d retries", maxRetries)
}
```

#### Circuit Breaker
```go
type CircuitBreaker struct {
    failures int
    lastFailure time.Time
    threshold int
    timeout time.Duration
}

func (cb *CircuitBreaker) Call(fn func() error) error {
    if cb.failures >= cb.threshold {
        if time.Since(cb.lastFailure) < cb.timeout {
            return fmt.Errorf("circuit breaker open")
        }
        cb.failures = 0 // Reset após timeout
    }

    if err := fn(); err != nil {
        cb.failures++
        cb.lastFailure = time.Now()
        return err
    }

    cb.failures = 0
    return nil
}
```

## Performance e Otimizações

### Tuning de Concorrência

```go
// Ajustar baseado na capacidade da máquina
const MaxConcurrency = runtime.NumCPU() * 2

// Ou baseado no tipo de processamento
const MaxConcurrency = 50 // Para I/O intensivo
const MaxConcurrency = 4  // Para CPU intensivo
```

### Batching de Mensagens

```go
func (b *Subscribe) processBatch(messages []*Message) {
    // Processar múltiplas mensagens juntas
    var batch []interface{}
    for _, msg := range messages {
        batch = append(batch, msg.Value)
    }

    // Processamento em lote
    if err := b.batchHandler(batch); err != nil {
        // Processar individualmente se lote falhar
        for _, msg := range messages {
            b.processMessage(msg.ID, msg.Key, msg.Value)
        }
    }
}
```

### Polling Otimizado

```go
func (b *Subscribe) optimizedPolling() {
    backoff := time.Millisecond * 10
    maxBackoff := time.Second * 5

    for {
        messagesProcessed := b.processAvailableMessages()

        if messagesProcessed == 0 {
            // Exponential backoff quando não há mensagens
            time.Sleep(backoff)
            if backoff < maxBackoff {
                backoff *= 2
            }
        } else {
            // Reset backoff quando há atividade
            backoff = time.Millisecond * 10
        }
    }
}
```

## Monitoramento e Métricas

### Métricas Importantes

```go
type SubscriberMetrics struct {
    MessagesProcessed int64
    ProcessingErrors  int64
    AverageLatency    time.Duration
    CurrentConcurrency int
    ClaimSuccessRate  float64
}
```

### Coleta de Métricas

```go
func (b *Subscribe) collectMetrics() {
    ticker := time.NewTicker(time.Minute)
    defer ticker.Stop()

    for range ticker.C {
        info, _ := b.GetSubscriberInfo()

        metrics.Gauge("subscriber.process_count").Set(float64(info.ProcessCount))
        metrics.Gauge("subscriber.weight").Set(float64(info.Weight))
        metrics.Gauge("subscriber.active").Set(1)

        // Concorrência atual (slots usados)
        currentConcurrency := MaxConcurrency - len(b.semaphore)
        metrics.Gauge("subscriber.concurrency").Set(float64(currentConcurrency))
    }
}
```

### Health Checks

```go
func (b *Subscribe) healthCheck() error {
    // Verificar se subscriber está registrado
    info, err := b.GetSubscriberInfo()
    if err != nil {
        return fmt.Errorf("subscriber not registered: %w", err)
    }

    // Verificar se heartbeat é recente
    if time.Since(time.Unix(info.LastSeen, 0)) > time.Minute {
        return fmt.Errorf("heartbeat stale")
    }

    // Verificar componentes
    if err := b.Queue.Health(); err != nil {
        return fmt.Errorf("queue unhealthy: %w", err)
    }

    if err := b.Balancer.Health(); err != nil {
        return fmt.Errorf("balancer unhealthy: %w", err)
    }

    return nil
}
```

## Troubleshooting

### Problemas Comuns

#### Mensagens não são processadas
```go
// Verificar se subscriber está ativo
info, err := subscriber.GetSubscriberInfo()
if err != nil {
    log.Printf("Subscriber não registrado: %v", err)
}

// Verificar se há mensagens
messages, _ := queue.GetUnclaimedMessagesByKey("event_type")
log.Printf("Mensagens pendentes: %d", len(messages))

// Verificar balanceamento
active, _ := balancer.GetActiveSubscribers()
log.Printf("Subscribers ativos: %v", active)
```

#### Alta latência
```go
// Monitorar concorrência
currentSlots := MaxConcurrency - len(subscriber.semaphore)
log.Printf("Slots em uso: %d/%d", currentSlots, MaxConcurrency)

// Aumentar concorrência se necessário
const MaxConcurrency = 20 // Era 10
```

#### Memory leaks
```go
// Verificar se WaitGroup está sendo decrementado
log.Printf("Goroutines ativas: %d", runtime.NumGoroutine())

// Verificar se mensagens estão sendo marcadas como processadas
```

### Debug Tools

```go
func debugSubscriber(s *sub.Subscribe) {
    info, _ := s.GetSubscriberInfo()
    log.Printf("Subscriber ID: %s", s.GetSubscriberID())
    log.Printf("Messages processed: %d", info.ProcessCount)
    log.Printf("Weight: %d", info.Weight)
    log.Printf("Last seen: %s", time.Unix(info.LastSeen, 0))

    // Listar eventos registrados
    // (Necessário adicionar método getter se precisar)
}
```

## Melhores Práticas

### 1. Sempre Use Graceful Shutdown
```go
// ✅ Implementar signal handling
go func() {
    <-sigterm
    subscriber.Stop()
}()
```

### 2. Error Handling Robusto
```go
// ✅ Tratar erros sem parar processamento
func handleEvent(ctx sub.Ctx) error {
    defer func() {
        if r := recover(); r != nil {
            log.Printf("Panic em handler: %v", r)
        }
    }()

    // Processamento com timeout
    ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
    defer cancel()

    return processWithTimeout(ctx, ctx.GetPayload())
}
```

### 3. Monitoramento Adequado
```go
// ✅ Métricas e health checks
go subscriber.collectMetrics()
go subscriber.periodicHealthCheck()
```

### 4. Configuração de Peso Apropriada
```go
// ✅ Baseado na capacidade real
weight := calculateInstanceCapacity()
subscriber.SetWeight(weight)
```

### 5. Handlers Idempotentes
```go
// ✅ Handlers devem ser seguros para reprocessamento
func handlePayment(ctx sub.Ctx) error {
    payment := ctx.GetPayload()

    // Verificar se já foi processado
    if paymentService.IsProcessed(payment.ID) {
        return nil // Já processado, sucesso
    }

    // Processar apenas se não foi processado
    return paymentService.Process(payment)
}
```
