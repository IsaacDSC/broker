# Broker - Sistema de Mensageria com Balanceamento de Carga

Um sistema de broker de mensagens em Go que suporta múltiplas implementações (memória e Redis) com balanceamento de carga automático entre consumidores.

## 🚀 Características

- **Múltiplas Implementações**: Suporte para armazenamento em memória e Redis
- **Balanceamento de Carga**: Distribuição automática de mensagens entre consumidores
- **Estratégias de Balanceamento**: Round Robin, Consistent Hash, Weighted, Least Connections
- **Zero Duplicação**: Garante que cada mensagem seja processada apenas uma vez
- **Thread-Safe**: Operações seguras para concorrência
- **Health Checks**: Monitoramento da saúde dos componentes
- **Configuração Flexível**: Via variáveis de ambiente ou código
- **Docker Ready**: Suporte completo para containers

## 📦 Instalação

```bash
git clone <repository-url>
cd broker
go mod tidy
```

## 🔧 Uso Básico

### Configuração em Memória (Padrão)

```go
package main

import (
    "github.com/IsaacDSC/broker/broker"
    "github.com/IsaacDSC/broker/pub"
    "github.com/IsaacDSC/broker/sub"
)

func main() {
    // Criar cliente com backend em memória
    client, err := broker.NewMemoryClient("my-app")
    if err != nil {
        panic(err)
    }
    defer client.Close()

    // Configurar consumidor
    go setupConsumer(client)

    // Configurar produtor
    setupProducer(client)
}

func setupConsumer(client *broker.Client) {
    subscriber := sub.NewSubscribe(client).
        WithSubscriber("user.created", func(ctx sub.Ctx) error {
            payload := ctx.GetPayload()
            fmt.Printf("Processando usuário: %v\n", payload)
            return nil
        })

    subscriber.Listener()
}

func setupProducer(client *broker.Client) {
    publisher := pub.NewBroker(client)

    err := publisher.Publish("user.created", map[string]any{
        "id":   123,
        "name": "João Silva",
        "email": "joao@example.com",
    })

    if err != nil {
        fmt.Printf("Erro ao publicar: %v\n", err)
    }
}
```

### Configuração com Redis

```go
// Criar cliente com backend Redis
client, err := broker.NewRedisClient("my-app", "localhost:6379")
if err != nil {
    panic(err)
}
defer client.Close()
```

### Configuração Híbrida

```go
// Queue no Redis, Balancer em memória
client, err := broker.NewHybridClient("my-app", "localhost:6379")
if err != nil {
    panic(err)
}
defer client.Close()
```

## ⚙️ Configuração Avançada

### Via Variáveis de Ambiente

```bash
# Tipo de backend
export BROKER_MODE=redis          # memory, redis, hybrid
export REDIS_ADDR=localhost:6379

# Configurações da Queue
export QUEUE_TYPE=redis
export QUEUE_PREFIX=my_queue
export QUEUE_REDIS_ADDR=localhost:6379
export QUEUE_REDIS_DB=0
export QUEUE_REDIS_POOL_SIZE=20

# Configurações do Balancer
export BALANCER_TYPE=redis
export BALANCER_PREFIX=my_balancer
export BALANCE_STRATEGY=consistent_hash  # round_robin, consistent_hash, weighted, least_conn
export HEARTBEAT_TTL=30s
export CLEANUP_TICKER=60s
```

### Via Código

```go
import (
    "github.com/IsaacDSC/broker/config"
    "github.com/IsaacDSC/broker/load"
)

cfg := &config.Config{
    AppName: "my-app",
    Queue: &config.QueueConfig{
        Type: "redis",
        Redis: &config.RedisConfig{
            Addr:     "localhost:6379",
            Password: "",
            DB:       0,
            PoolSize: 20,
        },
    },
    Balancer: &config.BalancerConfig{
        Type:     "redis",
        Strategy: load.StrategyWeighted,
        Redis: &config.RedisConfig{
            Addr: "localhost:6379",
            DB:   1,
        },
    },
}

client, err := broker.NewClientWithConfig(cfg)
```

## 🔄 Estratégias de Balanceamento

### Round Robin
Distribui mensagens sequencialmente entre consumidores.
```bash
export BALANCE_STRATEGY=round_robin
```

### Consistent Hash
Distribui mensagens baseado no hash do conteúdo.
```bash
export BALANCE_STRATEGY=consistent_hash
```

### Weighted
Distribui mensagens baseado no peso dos consumidores.
```go
subscriber.SetWeight(3) // Este consumidor receberá mais mensagens
```

### Least Connections
Distribui para o consumidor com menos mensagens processadas.
```bash
export BALANCE_STRATEGY=least_conn
```

## 🐳 Docker

### Executar com Docker Compose

```bash
# Subir Redis
docker-compose up redis -d

# Testar aplicação com memória
docker-compose up broker-app-memory

# Testar aplicação com Redis
docker-compose up broker-app-redis

# Testar aplicação híbrida
docker-compose up broker-app-hybrid

# Testar múltiplas instâncias (balanceamento real)
docker-compose up broker-instance-1 broker-instance-2 broker-instance-3 broker-producer
```

### Monitoramento

```bash
# Redis Commander (Interface web para Redis)
docker-compose up redis-commander
# Acesse: http://localhost:8081

# Monitor do Broker
docker-compose up broker-monitor
# Acesse: http://localhost:8080
```

## 📊 Monitoramento e Estatísticas

```go
// Verificar saúde
err := client.Health()

// Obter estatísticas
stats, err := client.GetStats()
fmt.Printf("Mensagens na fila: %d\n", stats["queue_length"])
fmt.Printf("Consumidores ativos: %d\n", stats["active_subscribers"])

// Informações do subscriber
info, err := subscriber.GetSubscriberInfo()
```

## 🧪 Exemplos de Uso

### Executar exemplos locais

```bash
# Backend em memória
BROKER_MODE=memory go run main.go

# Backend Redis (precisa do Redis rodando)
BROKER_MODE=redis go run main.go

# Backend híbrido
BROKER_MODE=hybrid go run main.go
```

### Teste de múltiplas instâncias

```bash
# Terminal 1 - Redis
docker run -p 6379:6379 redis:alpine

# Terminal 2 - Instância 1
BROKER_MODE=redis INSTANCE_NAME=worker-1 go run examples/consumer/main.go

# Terminal 3 - Instância 2
BROKER_MODE=redis INSTANCE_NAME=worker-2 go run examples/consumer/main.go

# Terminal 4 - Producer
BROKER_MODE=redis go run examples/producer/main.go
```

## 🔧 Configurações de Performance

### Para alta throughput

```bash
export QUEUE_REDIS_POOL_SIZE=50
export QUEUE_REDIS_MAX_RETRIES=5
export BALANCER_TYPE=memory  # Mais rápido que Redis
export BALANCE_STRATEGY=round_robin  # Mais simples
```

### Para alta disponibilidade

```bash
export QUEUE_TYPE=redis
export BALANCER_TYPE=redis
export BALANCE_STRATEGY=consistent_hash
export HEARTBEAT_TTL=15s
export CLEANUP_TICKER=30s
```

## 🚨 Solução de Problemas

### Mensagens duplicadas
- Verifique se está usando a mesma `AppName` em todas as instâncias
- Confirme que o balanceamento está funcionando: `client.GetStats()`

### Consumidores não recebem mensagens
- Verifique se os subscribers estão registrados: `GetActiveSubscribers()`
- Confirme a saúde dos componentes: `client.Health()`

### Problemas de conexão Redis
- Teste a conexão: `redis-cli -h host -p port ping`
- Verifique as configurações de rede e firewall
- Monitore logs do Redis

### Performance baixa
- Aumente o `PoolSize` do Redis
- Use balancer em memória para maior velocidade
- Considere usar múltiplos bancos Redis (sharding)

## 📚 API Reference

### Interfaces Principais

#### Queue Interface
```go
type Queue interface {
    Store(key string, value any) error
    Load(key string) (any, bool)
    Delete(key string) error
    ClaimMessage(messageID string) bool
    MarkAsProcessed(messageID string) error
    GetUnclaimedMessagesByKey(key string) ([]*Message, error)
    Close() error
    Health() error
}
```

#### Balancer Interface
```go
type Balancer interface {
    Subscribe(subID uuid.UUID) error
    Unsubscribe(subID uuid.UUID) error
    ClaimMessage(subID uuid.UUID, messageID string) (bool, error)
    GetActiveSubscribers() ([]uuid.UUID, error)
    SetSubscriberWeight(subID uuid.UUID, weight int) error
    Close() error
    Health() error
}
```

## 🤝 Contribuindo

1. Fork o projeto
2. Crie uma branch para sua feature (`git checkout -b feature/AmazingFeature`)
3. Commit suas mudanças (`git commit -m 'Add some AmazingFeature'`)
4. Push para a branch (`git push origin feature/AmazingFeature`)
5. Abra um Pull Request

## 📝 Licença

Este projeto está licenciado sob a MIT License - veja o arquivo [LICENSE](LICENSE) para detalhes.

## 🔄 Changelog

### v2.0.0
- ✨ Implementação de interfaces para Queue e Balancer
- ✨ Suporte completo ao Redis
- ✨ Múltiplas estratégias de balanceamento
- ✨ Sistema de configuração flexível
- ✨ Docker support completo
- 🐛 Correção de duplicação de mensagens
- ⚡ Melhorias de performance

### v1.0.0
- 🎉 Versão inicial com suporte básico
- ✨ Queue em memória
- ✨ Balanceamento round-robin básico

## 📞 Suporte

- 📧 Email: support@broker.com
- 💬 Discord: [Link do Discord]
- 📖 Documentação: [Link da Docs]
- 🐛 Issues: [GitHub Issues](https://github.com/user/broker/issues)
