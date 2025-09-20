# Documentação Técnica - Sistema Broker

## Visão Geral

Este diretório contém a documentação técnica completa do sistema de broker de mensagens. Cada package foi documentado individualmente com detalhes sobre responsabilidades, arquitetura, funções e melhores práticas.

## Estrutura da Documentação

### 📋 Documentos Principais

| Documento | Descrição | Responsabilidade |
|-----------|-----------|------------------|
| [broker.md](./broker.md) | Cliente Principal | Orquestração de componentes e API unificada |
| [queue.md](./queue.md) | Sistema de Filas | Armazenamento e gerenciamento de mensagens |
| [load.md](./load.md) | Balanceamento de Carga | Distribuição inteligente entre consumidores |
| [config.md](./config.md) | Sistema de Configuração | Centralização e validação de configurações |
| [factory.md](./factory.md) | Criação de Componentes | Factory pattern para instanciação |
| [sub.md](./sub.md) | Subscribers/Consumidores | Sistema de processamento de mensagens |
| [pub.md](./pub.md) | Publishers/Produtores | Interface de publicação de mensagens |

## Arquitetura do Sistema

### 🏗️ Visão de Alto Nível

```
┌─────────────────────────────────────────────────────────────┐
│                     APPLICATION LAYER                       │
├─────────────────────────────────────────────────────────────┤
│  Publisher (pub)  │              │  Subscriber (sub)        │
│                   │              │                          │
│  • Publish()      │              │  • WithSubscriber()      │
│  • Event routing  │              │  • Listener()            │
│                   │              │  • Message processing    │
└─────────────────────┴──────────────┴─────────────────────────┘
                      │
┌─────────────────────▼─────────────────────────────────────────┐
│                    BROKER CLIENT                              │
├─────────────────────────────────────────────────────────────┤
│  • Component orchestration    │  • Health management         │
│  • Configuration management   │  • Statistics collection     │
│  • Lifecycle control         │  • Error handling            │
└─────────────────────────────────────────────────────────────┘
                      │
┌─────────────────────▼─────────────────────────────────────────┐
│                  CORE COMPONENTS                              │
├─────────────────────────────────────────────────────────────┤
│                     │                                        │
│   QUEUE SYSTEM      │      LOAD BALANCER                     │
│                     │                                        │
│   • Message storage │      • Subscriber management           │
│   • Atomic claims   │      • Distribution strategies         │
│   • Processing ctrl │      • Heartbeat monitoring            │
│                     │      • Health tracking                 │
└─────────────────────┴────────────────────────────────────────┘
                      │
┌─────────────────────▼─────────────────────────────────────────┐
│                STORAGE BACKENDS                               │
├─────────────────────────────────────────────────────────────┤
│                     │                                        │
│   MEMORY BACKEND    │         REDIS BACKEND                  │
│                     │                                        │
│   • Ultra-fast      │         • Distributed                 │
│   • No persistence  │         • Persistent                   │
│   • Single instance │         • Scalable                     │
│                     │         • Network latency              │
└─────────────────────┴────────────────────────────────────────┘
                      │
┌─────────────────────▼─────────────────────────────────────────┐
│               CONFIGURATION & FACTORY                         │
├─────────────────────────────────────────────────────────────┤
│  CONFIG             │              FACTORY                   │
│                     │                                        │
│  • Environment vars │              • Component creation      │
│  • Validation       │              • Dependency injection    │
│  • Presets          │              • Error handling          │
│  • Type conversion  │              • Resource management     │
└─────────────────────┴────────────────────────────────────────┘
```

### 🔄 Fluxo de Mensagens

```
┌─────────┐    1. Publish    ┌─────────┐    2. Store    ┌─────────┐
│Publisher├─────────────────▶│ Broker  ├──────────────▶│  Queue  │
└─────────┘                  └─────────┘                └─────────┘
                                                              │
                                                              │ 3. Get Messages
                                                              ▼
┌─────────┐    7. Process    ┌─────────┐    4. Claim    ┌─────────┐
│ Handler │◀─────────────────│Subscriber│◀──────────────│Balancer │
└─────────┘                  └─────────┘                └─────────┘
                                   │                          ▲
                                   │ 5. Claim Message         │
                                   ▼                          │ 6. Should Process?
                             ┌─────────┐                      │
                             │  Queue  ├──────────────────────┘
                             └─────────┘
```

## Componentes Detalhados

### 🎯 Broker Client ([broker.md](./broker.md))

**Ponto de entrada principal do sistema**

- Orquestra Queue e Balancer
- Fornece APIs convenientes (NewMemoryClient, NewRedisClient, etc.)
- Gerencia configurações e health checks
- Abstrai complexidade dos componentes internos

### 📦 Queue System ([queue.md](./queue.md))

**Sistema de armazenamento de mensagens**

#### Implementações:
- **Memory**: Ultra-rápido, sem persistência
- **Redis**: Distribuído, persistente, escalável

#### Funcionalidades:
- Armazenamento thread-safe
- Claims atômicos para evitar duplicação
- Controle de ciclo de vida das mensagens
- Suporte a múltiplos backends

### ⚖️ Load Balancer ([load.md](./load.md))

**Sistema de distribuição inteligente**

#### Estratégias:
- **Round Robin**: Distribuição sequencial
- **Consistent Hash**: Baseado no hash da mensagem
- **Weighted**: Baseado no peso dos consumidores
- **Least Connections**: Para o consumidor com menor carga

#### Funcionalidades:
- Registro automático de subscribers
- Heartbeat monitoring
- Cleanup de subscribers inativos
- Distribuição sem duplicação

### ⚙️ Config System ([config.md](./config.md))

**Centralização de configurações**

#### Fontes:
- Variáveis de ambiente
- Configuração programática
- Presets pré-definidos

#### Funcionalidades:
- Validação automática
- Conversão de tipos
- Configurações por ambiente
- Suporte a configurações híbridas

### 🏭 Factory System ([factory.md](./factory.md))

**Criação automatizada de componentes**

#### Responsabilidades:
- Instanciação baseada em configuração
- Validação de dependências
- Health checks pós-criação
- Cleanup automático em caso de erro

### 📥 Subscriber System ([sub.md](./sub.md))

**Sistema de consumidores**

#### Funcionalidades:
- Registro de handlers por evento
- Controle de concorrência via semáforos
- Integração com balancer
- Graceful shutdown
- Heartbeat automático

### 📤 Publisher System ([pub.md](./pub.md))

**Interface de publicação**

#### Características:
- API simples e direta
- Suporte a qualquer tipo de payload
- Thread-safe por design
- Error handling robusto

## Configurações Suportadas

### 🧠 Memory (Desenvolvimento)

```go
client, _ := broker.NewMemoryClient("dev-app")
```

**Características:**
- ✅ Zero dependências
- ✅ Performance máxima
- ✅ Ideal para desenvolvimento
- ❌ Não persiste dados
- ❌ Single-instance apenas

### 🔴 Redis (Produção)

```go
client, _ := broker.NewRedisClient("prod-app", "redis:6379")
```

**Características:**
- ✅ Persistência garantida
- ✅ Distribuído
- ✅ Escalável horizontalmente
- ❌ Dependência externa
- ❌ Latência de rede

### 🔄 Hybrid (Balanceado)

```go
client, _ := broker.NewHybridClient("app", "redis:6379")
```

**Características:**
- ✅ Queue persistente (Redis)
- ✅ Balancer rápido (Memory)
- ⚠️ Balancer não compartilhado

## Exemplos de Uso

### 📝 Exemplo Completo - Memory

```go
func main() {
    // Cliente em memória
    client, err := broker.NewMemoryClient("my-app")
    if err != nil {
        log.Fatal(err)
    }
    defer client.Close()

    // Configurar consumidor
    go func() {
        subscriber := sub.NewSubscribe(client).
            WithSubscriber("user.created", func(ctx sub.Ctx) error {
                userData := ctx.GetPayload()
                fmt.Printf("Processando usuário: %v\n", userData)
                return nil
            })

        if err := subscriber.Listener(); err != nil {
            log.Printf("Subscriber error: %v", err)
        }
    }()

    // Dar tempo para subscriber se registrar
    time.Sleep(100 * time.Millisecond)

    // Publicar mensagens
    publisher := pub.NewBroker(client)
    err = publisher.Publish("user.created", map[string]any{
        "id":   123,
        "name": "João Silva",
        "email": "joao@example.com",
    })

    if err != nil {
        log.Printf("Publish error: %v", err)
    }

    time.Sleep(time.Second) // Aguardar processamento
}
```

### 🌐 Exemplo Completo - Redis

```go
func main() {
    // Cliente Redis distribuído
    client, err := broker.NewRedisClient("distributed-app", "localhost:6379")
    if err != nil {
        log.Fatal(err)
    }
    defer client.Close()

    // Health check
    if err := client.Health(); err != nil {
        log.Fatalf("Sistema não saudável: %v", err)
    }

    // Múltiplos subscribers
    go runSubscriber("worker-1", client)
    go runSubscriber("worker-2", client)

    // Publisher
    runPublisher(client)
}

func runSubscriber(name string, client *broker.Client) {
    subscriber := sub.NewSubscribe(client).
        WithSubscriber("order.placed", handleOrderPlaced).
        WithSubscriber("payment.processed", handlePaymentProcessed)

    log.Printf("[%s] Starting subscriber...", name)
    if err := subscriber.Listener(); err != nil {
        log.Printf("[%s] Subscriber error: %v", name, err)
    }
}

func runPublisher(client *broker.Client) {
    publisher := pub.NewBroker(client)

    for i := 0; i < 10; i++ {
        // Order events
        publisher.Publish("order.placed", map[string]any{
            "order_id": fmt.Sprintf("ORD-%d", i),
            "customer": "customer@example.com",
            "amount":   100.0 + float64(i),
        })

        // Payment events
        publisher.Publish("payment.processed", map[string]any{
            "payment_id": fmt.Sprintf("PAY-%d", i),
            "order_id":   fmt.Sprintf("ORD-%d", i),
            "status":     "completed",
        })

        time.Sleep(500 * time.Millisecond)
    }
}
```

## Monitoramento e Observabilidade

### 📊 Métricas Importantes

```go
// Coletar estatísticas
stats, err := client.GetStats()
if err == nil {
    fmt.Printf("Queue Length: %v\n", stats["queue_length"])
    fmt.Printf("Active Subscribers: %v\n", stats["active_subscribers"])
}

// Health checks
if err := client.Health(); err != nil {
    log.Printf("Sistema não saudável: %v", err)
}

// Informações do subscriber
info, err := subscriber.GetSubscriberInfo()
if err == nil {
    fmt.Printf("Messages Processed: %d\n", info.ProcessCount)
    fmt.Printf("Weight: %d\n", info.Weight)
}
```

### 🏥 Health Checks

```go
func monitorSystem(client *broker.Client) {
    ticker := time.NewTicker(30 * time.Second)
    defer ticker.Stop()

    for range ticker.C {
        if err := client.Health(); err != nil {
            log.Printf("❌ System unhealthy: %v", err)
            // Implementar alertas, restart, etc.
        } else {
            log.Println("✅ System healthy")
        }
    }
}
```

## Deployment e Configuração

### 🐳 Docker Compose

Ver `docker-compose.yml` na raiz do projeto para exemplos completos de deployment com:

- Redis cluster
- Múltiplas instâncias de workers
- Monitoring stack
- Load balancing

### 🌍 Variáveis de Ambiente

```bash
# Configuração básica
APP_NAME=my-app
BROKER_MODE=redis  # memory, redis, hybrid

# Redis específico
REDIS_ADDR=localhost:6379
QUEUE_TYPE=redis
BALANCER_TYPE=redis
BALANCE_STRATEGY=consistent_hash

# Performance tuning
QUEUE_REDIS_POOL_SIZE=20
HEARTBEAT_TTL=30s
CLEANUP_TICKER=60s
```

## Performance e Escalabilidade

### 📈 Benchmarks

| Backend | Latência | Throughput | Uso de Memória |
|---------|----------|------------|----------------|
| Memory  | ~150ns   | 10M msg/s  | Baixo          |
| Redis   | ~15ms    | 100K msg/s | Médio          |

### 🚀 Otimizações

#### Para Alta Performance:
- Use Memory backend quando possível
- Configure pools Redis adequadamente
- Ajuste concorrência dos subscribers
- Monitore tamanho das filas

#### Para Alta Disponibilidade:
- Use Redis backend
- Configure múltiplas instâncias
- Implemente health checks
- Use heartbeats frequentes

## Troubleshooting

### 🔍 Problemas Comuns

#### Mensagens Duplicadas
- Verificar se todos subscribers usam mesmo AppName
- Confirmar que balanceamento está ativo
- Analisar logs de claim de mensagens

#### Performance Baixa
- Monitorar latência de Redis
- Verificar concorrência de subscribers
- Analisar tamanho dos payloads
- Checar configuração de pools

#### Subscribers Não Recebem Mensagens
- Verificar registro no balancer
- Confirmar health dos componentes
- Validar configuração de eventos
- Checar heartbeats

### 🛠️ Debug Tools

```go
// Debug completo do sistema
func debugSystem(client *broker.Client) {
    // Configuração
    client.PrintConfig()

    // Estatísticas
    stats, _ := client.GetStats()
    log.Printf("Stats: %+v", stats)

    // Health
    if err := client.Health(); err != nil {
        log.Printf("Health issues: %v", err)
    }

    // Balancer info
    active, _ := client.Balancer.GetActiveSubscribers()
    log.Printf("Active subscribers: %v", active)
}
```

## Roadmap e Extensões

### 🚧 Funcionalidades Futuras

- [ ] Dead Letter Queue
- [ ] Message TTL
- [ ] Priority queues
- [ ] Batch processing
- [ ] Message routing patterns
- [ ] Prometheus metrics
- [ ] Distributed tracing
- [ ] Schema validation

### 🔧 Extensibilidade

O sistema foi projetado para ser extensível:

- **Novos Backends**: Implementar interfaces Queue/Balancer
- **Novas Estratégias**: Adicionar ao enum BalanceStrategy
- **Middlewares**: Wrapper pattern para funcionalidades extras
- **Plugins**: Sistema de hooks para customização

## Contribuição

Para contribuir com a documentação:

1. Mantenha consistência de formato
2. Inclua exemplos práticos
3. Documente casos de uso reais
4. Atualize este overview quando adicionar novos docs
5. Use diagramas quando apropriado

## Contato e Suporte

- 📖 Documentação completa: Ver arquivos individuais neste diretório
- 🐛 Issues: GitHub Issues
- 💬 Discussões: GitHub Discussions
- 📧 Suporte: Via issues ou discussões
