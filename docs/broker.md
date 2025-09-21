# Package Broker - Cliente Principal do Sistema

## Visão Geral

O package `broker` é o ponto de entrada principal do sistema de mensageria. Ele atua como uma fachada que orquestra os componentes de Queue e Balancer, fornecendo uma API simplificada para criação de clientes com diferentes configurações de backend (memória, Redis, híbrido).

## Responsabilidades Principais

- **Orquestração de Componentes**: Coordena Queue e Balancer em uma interface unificada
- **Gerenciamento de Configuração**: Aplica configurações específicas aos componentes
- **Abstração de Complexidade**: Esconde detalhes de implementação dos usuários
- **Lifecycle Management**: Controla inicialização, saúde e encerramento dos componentes
- **API Unificada**: Fornece interface consistente independente do backend
- **Factory Management**: Facilita criação de diferentes tipos de cliente

## Arquitetura

### Componentes Integrados

- **Queue**: Gerenciamento de mensagens e persistência
- **Balancer**: Distribuição e balanceamento de carga
- **Config**: Sistema de configuração flexível
- **Factory**: Criação automatizada de componentes

### `NewMemoryClient(appName string) (*Client, error)`

**Responsabilidade**: Cria cliente usando apenas memória
**Parâmetros**:

- `appName`: Nome da aplicação
  **Retorno**: Cliente configurado para memória
  **Comportamento**:
- Queue e Balancer em memória
- Performance máxima, sem persistência
- Ideal para desenvolvimento e testes

**Características**:

- ✅ Ultra-baixa latência
- ✅ Sem dependências externas
- ❌ Não persiste entre reinicializações
- ❌ Limitado a single-instance

### `NewRedisClient(appName, redisAddr string) (*Client, error)`

**Responsabilidade**: Cria cliente usando Redis completo
**Parâmetros**:

- `appName`: Nome da aplicação
- `redisAddr`: Endereço do servidor Redis
  **Retorno**: Cliente configurado para Redis
  **Comportamento**:
- Queue e Balancer no Redis
- Persistência e distribuição garantidas
- Ideal para produção

**Características**:

- ✅ Persistência de dados
- ✅ Compartilhamento entre instâncias
- ✅ Escalabilidade horizontal
- ❌ Dependência externa (Redis)
- ❌ Latência de rede

### `NewHybridClient(appName, redisAddr string) (*Client, error)`

**Responsabilidade**: Cria cliente híbrido (Redis Queue + Memory Balancer)
**Parâmetros**:

- `appName`: Nome da aplicação
- `redisAddr`: Endereço do Redis (para Queue)
  **Retorno**: Cliente com configuração híbrida
  **Comportamento**:
- Queue no Redis (persistência)
- Balancer em memória (performance)
- Balanceamento entre persistência e velocidade

**Características**:

- ✅ Mensagens persistem (Queue Redis)
- ✅ Balanceamento rápido (Memory)
- ⚠️ Balancer não compartilhado entre instâncias
- 🎯 Ideal para casos específicos

### `Health() error`

**Responsabilidade**: Verifica saúde de todos os componentes
**Retorno**: Error se algum componente não está saudável
**Comportamento**:

- Checa Queue.Health()
- Checa Balancer.Health()
- Falha rápida se qualquer componente falhar

**Exemplo**:

```go
if err := client.Health(); err != nil {
    log.Printf("Sistema não saudável: %v", err)
    // Implementar recuperação ou alerta
}
```

### `GetStats() (map[string]interface{}, error)`

**Responsabilidade**: Coleta estatísticas de todos os componentes
**Retorno**: Mapa com métricas agregadas
**Comportamento**:

- Consulta Queue.Len() para tamanho da fila
- Consulta Balancer para subscribers ativos
- Agrega métricas em formato unificado

**Métricas Incluídas**:

```go
{
    "queue_length": 42,              // Mensagens pendentes
    "total_subscribers": 5,          // Total de subscribers
    "active_subscribers": 3,         // Subscribers ativos
}
```

### Aplicação Distribuída (Produção)

```go
func main() {
    // Cliente Redis para distribuição
    client, err := broker.NewRedisClient("prod-app", "redis:6379")
    if err != nil {
        log.Fatal(err)
    }
    defer client.Close()

    // Health check
    if err := client.Health(); err != nil {
        log.Fatalf("Sistema não saudável: %v", err)
    }

    // Monitoramento periódico
    go monitorHealth(client)

    // Lógica da aplicação
    runApplication(client)
}
```

### Configuração Avançada

```go
func createAdvancedClient() (*broker.Client, error) {
    cfg := &config.Config{
        AppName: "advanced-app",
        Queue: &config.QueueConfig{
            Type: queue.QueueTypeRedis,
            Redis: &config.RedisConfig{
                Addr:         "redis-cluster:6379",
                Password:     os.Getenv("REDIS_PASSWORD"),
                DB:           1,
                PoolSize:     20,
                MaxRetries:   5,
                MinIdleConns: 5,
            },
            Prefix: "prod_queue",
        },
        Balancer: &config.BalancerConfig{
            Type:          load.BalancerTypeRedis,
            Strategy:      load.StrategyWeighted,
            HeartbeatTTL:  45 * time.Second,
            CleanupTicker: 2 * time.Minute,
            Redis: &config.RedisConfig{
                Addr:     "redis-cluster:6379",
                DB:       2,
                PoolSize: 15,
            },
            Prefix: "prod_balancer",
        },
    }

    return broker.NewClientWithConfig(cfg)
}
```

## Integração com Outros Packages

### Com Package Sub (Subscribers)

```go
client, _ := broker.NewRedisClient("app", "localhost:6379")

subscriber := sub.NewSubscribe(client).
    WithSubscriber("user.created", handleUserCreated).
    WithSubscriber("user.updated", handleUserUpdated)

go subscriber.Listener()
```

### Com Package Pub (Publishers)

```go
client, _ := broker.NewRedisClient("app", "localhost:6379")

publisher := pub.NewBroker(client)
err := publisher.Publish("user.created", userData)
```

### Com Package Config

```go
// Via variáveis de ambiente
cfg := config.LoadConfig()
client, _ := broker.NewClientWithConfig(cfg)

// Via configuração programática
cfg := config.NewRedisConfig("app", "localhost:6379")
client, _ := broker.NewClientWithConfig(cfg)
```

## Monitoramento e Observabilidade

### Health Checks Periódicos

```go
func monitorHealth(client *broker.Client) {
    ticker := time.NewTicker(30 * time.Second)
    defer ticker.Stop()

    for range ticker.C {
        if err := client.Health(); err != nil {
            log.Printf("❌ Health check failed: %v", err)
            // Alertas, métricas, etc.
        } else {
            log.Println("✅ System healthy")
        }
    }
}
```

### Métricas Customizadas

```go
func collectMetrics(client *broker.Client) {
    stats, err := client.GetStats()
    if err != nil {
        log.Printf("Erro coletando métricas: %v", err)
        return
    }

    // Enviar para sistema de métricas (Prometheus, etc.)
    prometheus.QueueLength.Set(float64(stats["queue_length"].(int)))
    prometheus.ActiveSubscribers.Set(float64(stats["active_subscribers"].(int)))
}
```

## Configurações por Ambiente

### Desenvolvimento

```go
// Rápido, simples, sem dependências
client, _ := broker.NewMemoryClient("dev-app")
```

### Teste/Staging

```go
// Redis local com configurações moderadas
client, _ := broker.NewRedisClient("staging-app", "localhost:6379")
```

### Produção

```go
// Redis cluster com configurações otimizadas
cfg := config.NewRedisConfig("prod-app", "redis-cluster:6379")
cfg.Queue.Redis.PoolSize = 50
cfg.Queue.Redis.MaxRetries = 10
cfg.Balancer.Strategy = load.StrategyConsistentHash
cfg.Balancer.HeartbeatTTL = 30 * time.Second

client, _ := broker.NewClientWithConfig(cfg)
```

### Middleware Pattern

```go
type ClientMiddleware func(*Client) *Client

func WithLogging(client *Client) *Client {
    // Wrap operations with logging
    return client
}

func WithMetrics(client *Client) *Client {
    // Wrap operations with metrics
    return client
}
```
