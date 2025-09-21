# Package Config - Sistema de Configuração

## Visão Geral

O package `config` é responsável pela centralização e gerenciamento de todas as configurações do sistema de broker. Ele fornece uma interface unificada para configurar diferentes componentes (Queue e Balancer) através de múltiplas fontes (variáveis de ambiente, código, configurações pré-definidas), garantindo flexibilidade e facilidade de uso.

## Responsabilidades Principais

- **Centralização de Configurações**: Unifica configurações de todos os componentes em uma estrutura
- **Múltiplas Fontes**: Suporte para variáveis de ambiente, código e presets
- **Validação**: Valida consistência e completude das configurações
- **Configurações Pré-definidas**: Fornece templates para diferentes cenários
- **Conversão de Tipos**: Transforma configurações para formatos específicos dos componentes
- **Environment-Aware**: Adapta configurações base
  ado no ambiente de execução

## Arquitetura

### Estrutura Principal

```go
type Config struct {
    AppName  string           // Nome identificador da aplicação
    Queue    *QueueConfig     // Configurações da fila
    Balancer *BalancerConfig  // Configurações do balanceador
}
```

### Componentes de Configuração

#### QueueConfig

```go
type QueueConfig struct {
    Type   queue.QueueType  // Tipo: memory ou redis
    Redis  *RedisConfig     // Configurações Redis (se aplicável)
    Prefix string          // Prefixo para namespacing
}
```

#### BalancerConfig

```go
type BalancerConfig struct {
    Type          load.BalancerType    // Tipo: memory ou redis
    Redis         *RedisConfig         // Configurações Redis (se aplicável)
    Strategy      load.BalanceStrategy // Estratégia de balanceamento
    HeartbeatTTL  time.Duration       // TTL do heartbeat
    CleanupTicker time.Duration       // Intervalo de limpeza
    Prefix        string              // Prefixo para namespacing
}
```

#### RedisConfig

```go
type RedisConfig struct {
    Addr         string  // Endereço do servidor Redis
    Password     string  // Senha de autenticação
    DB           int     // Número do banco de dados
    PoolSize     int     // Tamanho do pool de conexões
    MaxRetries   int     // Número máximo de tentativas
    MinIdleConns int     // Conexões mínimas idle no pool
}
```

## Enums e Constantes

### QueueType

```go
const (
    QueueTypeMemory QueueType = "memory"  // Fila em memória
    QueueTypeRedis  QueueType = "redis"   // Fila no Redis
)
```

### BalancerType

```go
const (
    BalancerTypeMemory BalancerType = "memory"  // Balancer em memória
    BalancerTypeRedis  BalancerType = "redis"   // Balancer no Redis
)
```

### BalanceStrategy

```go
const (
    StrategyRoundRobin     BalanceStrategy = "round_robin"      // Round robin
    StrategyConsistentHash BalanceStrategy = "consistent_hash"  // Hash consistente
    StrategyWeighted       BalanceStrategy = "weighted"         // Baseado em peso
    StrategyLeastConn      BalanceStrategy = "least_conn"       // Menor conexão
)
```

## Funções Principais

### `LoadConfig() *Config`

**Responsabilidade**: Carrega configurações de variáveis de ambiente
**Retorno**: Configuração completa baseada no ambiente
**Comportamento**:

- Lê variáveis de ambiente com fallbacks para defaults
- Aplica configurações específicas por componente
- Configura automaticamente Redis se necessário

**Variáveis de Ambiente Suportadas**:

```bash
# Configurações Gerais
APP_NAME=my-app

# Configurações da Queue
QUEUE_TYPE=redis
QUEUE_PREFIX=queue
QUEUE_REDIS_ADDR=localhost:6379
QUEUE_REDIS_PASSWORD=secret
QUEUE_REDIS_DB=0
QUEUE_REDIS_POOL_SIZE=10
QUEUE_REDIS_MAX_RETRIES=3
QUEUE_REDIS_MIN_IDLE_CONNS=2

# Configurações do Balancer
BALANCER_TYPE=redis
BALANCER_PREFIX=balancer
BALANCE_STRATEGY=consistent_hash
HEARTBEAT_TTL=30s
CLEANUP_TICKER=60s
BALANCER_REDIS_ADDR=localhost:6379
BALANCER_REDIS_PASSWORD=secret
BALANCER_REDIS_DB=1
```

**Exemplo**:

```go
// Carrega do ambiente
config := config.LoadConfig()
client, err := broker.NewClientWithConfig(config)
```

### `NewMemoryConfig(appName string) *Config`

**Responsabilidade**: Cria configuração otimizada para memória
**Parâmetros**:

- `appName`: Nome da aplicação
  **Retorno**: Configuração com componentes em memória
  **Comportamento**:
- Queue em memória para máxima performance
- Balancer em memória com round robin
- Configurações padrão para desenvolvimento

**Características**:

- ✅ Zero dependências externas
- ✅ Inicialização instantânea
- ✅ Performance máxima
- ❌ Não persiste dados
- ❌ Limitado a single-instance

**Exemplo**:

```go
config := config.NewMemoryConfig("dev-app")
client, err := broker.NewClientWithConfig(config)
```

### `NewRedisConfig(appName, redisAddr string) *Config`

**Responsabilidade**: Cria configuração otimizada para Redis
**Parâmetros**:

- `appName`: Nome da aplicação
- `redisAddr`: Endereço do servidor Redis
  **Retorno**: Configuração com componentes no Redis
  **Comportamento**:
- Queue no Redis para persistência
- Balancer no Redis para distribuição
- Estratégia consistent hash por padrão
- Configurações otimizadas para produção

**Características**:

- ✅ Persistência garantida
- ✅ Compartilhamento entre instâncias
- ✅ Escalabilidade horizontal
- ❌ Dependência externa
- ❌ Latência de rede

**Exemplo**:

```go
config := config.NewRedisConfig("prod-app", "redis-cluster:6379")
client, err := broker.NewClientWithConfig(config)
```

### `NewHybridConfig(appName, redisAddr string) *Config`

**Responsabilidade**: Cria configuração híbrida (Redis Queue + Memory Balancer)
**Parâmetros**:

- `appName`: Nome da aplicação
- `redisAddr`: Endereço do Redis para Queue
  **Retorno**: Configuração híbrida otimizada
  **Comportamento**:
- Queue no Redis para persistência de mensagens
- Balancer em memória para performance de distribuição
- Estratégia round robin para simplicidade
- Balanceamento entre persistência e velocidade

**Características**:

- ✅ Mensagens persistem (Redis Queue)
- ✅ Distribuição rápida (Memory Balancer)
- ⚠️ Balancer não compartilhado entre instâncias
- 🎯 Ideal para casos específicos

**Exemplo**:

```go
config := config.NewHybridConfig("hybrid-app", "localhost:6379")
client, err := broker.NewClientWithConfig(config)
```

## Métodos de Configuração

### `(c *Config) Validate() error`

**Responsabilidade**: Valida consistência e completude da configuração
**Retorno**: Error se configuração inválida
**Validações Realizadas**:

- AppName não pode ser vazio
- Tipos de Queue devem ser válidos (memory/redis)
- Tipos de Balancer devem ser válidos (memory/redis)
- Configuração Redis obrigatória quando tipo é redis
- Estratégias de balanceamento devem ser válidas
- TTLs e intervalos devem ser positivos

**Exemplo**:

```go
config := &Config{...}
if err := config.Validate(); err != nil {
    log.Fatalf("Configuração inválida: %v", err)
}
```

### `(c *Config) Print()`

**Responsabilidade**: Imprime configuração atual de forma organizada
**Comportamento**:

- Formata informações de forma legível
- Mostra configurações ativas
- Útil para debug e verificação
- Não expõe informações sensíveis (senhas)

**Saída Exemplo**:

```
=== Broker Configuration ===
App Name: my-app
Queue Type: redis
Balancer Type: memory
Balance Strategy: round_robin
Heartbeat TTL: 30s
Cleanup Ticker: 1m0s
Queue Redis: localhost:6379 (DB: 0)
===========================
```

### Métodos de Conversão

#### `(qc *QueueConfig) ToQueueConfig(appName string) *queue.QueueConfig`

**Responsabilidade**: Converte para formato específico do package queue
**Parâmetros**:

- `appName`: Nome da aplicação para contexto
  **Retorno**: Configuração no formato esperado pelo queue package
  **Comportamento**:
- Transforma estrutura config para queue.QueueConfig
- Aplica namespacing com appName
- Converte configurações Redis se aplicável

#### `(bc *BalancerConfig) ToBalancerConfig(appName string) *load.BalancerConfig`

**Responsabilidade**: Converte para formato específico do package load
**Parâmetros**:

- `appName`: Nome da aplicação para contexto
  **Retorno**: Configuração no formato esperado pelo load package
  **Comportamento**:
- Transforma estrutura config para load.BalancerConfig
- Aplica namespacing com appName
- Converte configurações Redis e estratégias

## Funções Auxiliares

### `getEnvString(key, defaultValue string) string`

**Responsabilidade**: Lê string de variável de ambiente com fallback
**Parâmetros**:

- `key`: Nome da variável de ambiente
- `defaultValue`: Valor padrão se variável não existir
  **Retorno**: Valor da variável ou padrão

### `getEnvInt(key string, defaultValue int) int`

**Responsabilidade**: Lê inteiro de variável de ambiente com fallback
**Parâmetros**:

- `key`: Nome da variável de ambiente
- `defaultValue`: Valor padrão se variável não existir ou inválida
  **Retorno**: Valor convertido ou padrão

### `getEnvDuration(key string, defaultValue time.Duration) time.Duration`

**Responsabilidade**: Lê duração de variável de ambiente com fallback
**Parâmetros**:

- `key`: Nome da variável de ambiente (formato: "30s", "1m", etc.)
- `defaultValue`: Valor padrão se variável não existir ou inválida
  **Retorno**: Duração convertida ou padrão

## Padrões de Uso

### Configuração via Ambiente

```bash
# .env ou export
export APP_NAME=production-app
export QUEUE_TYPE=redis
export BALANCER_TYPE=redis
export QUEUE_REDIS_ADDR=redis-cluster:6379
export BALANCE_STRATEGY=weighted
export HEARTBEAT_TTL=45s
```

```go
// Aplicação
config := config.LoadConfig()
client, err := broker.NewClientWithConfig(config)
```

### Configuração Programática

```go
config := &config.Config{
    AppName: "custom-app",
    Queue: &config.QueueConfig{
        Type: queue.QueueTypeRedis,
        Redis: &config.RedisConfig{
            Addr:         "localhost:6379",
            Password:     "secret",
            DB:           1,
            PoolSize:     20,
            MaxRetries:   5,
            MinIdleConns: 5,
        },
        Prefix: "custom_queue",
    },
    Balancer: &config.BalancerConfig{
        Type:          load.BalancerTypeMemory,
        Strategy:      load.StrategyWeighted,
        HeartbeatTTL:  30 * time.Second,
        CleanupTicker: time.Minute,
        Prefix:        "custom_balancer",
    },
}

client, err := broker.NewClientWithConfig(config)
```

### Configuração por Ambiente de Execução

```go
func createConfigForEnvironment() *config.Config {
    env := os.Getenv("ENVIRONMENT")

    switch env {
    case "production":
        return config.NewRedisConfig("prod-app", "redis-prod:6379")
    case "staging":
        return config.NewRedisConfig("staging-app", "redis-staging:6379")
    case "development":
        return config.NewMemoryConfig("dev-app")
    default:
        return config.NewMemoryConfig("default-app")
    }
}
```

## Configurações por Cenário

### Desenvolvimento Local

```go
// Simples e rápido
config := config.NewMemoryConfig("dev-app")
```

**Características**:

- Sem dependências externas
- Inicialização instantânea
- Ideal para desenvolvimento e testes unitários

### Testes de Integração

```go
// Com Redis local para testes mais realistas
config := config.NewRedisConfig("test-app", "localhost:6379")
config.Queue.Redis.DB = 15  // DB separado para testes
config.Balancer.Redis.DB = 15
```

### Staging/QA

```go
config := config.NewRedisConfig("staging-app", "redis-staging:6379")
config.Balancer.Strategy = load.StrategyRoundRobin  // Simplicidade
config.Balancer.HeartbeatTTL = 30 * time.Second
```

### Produção

```go
config := config.NewRedisConfig("prod-app", "redis-cluster:6379")
config.Queue.Redis.PoolSize = 50
config.Queue.Redis.MaxRetries = 10
config.Balancer.Strategy = load.StrategyConsistentHash
config.Balancer.HeartbeatTTL = 15 * time.Second
config.Balancer.CleanupTicker = 30 * time.Second
```

### Alta Disponibilidade

```go
config := config.NewRedisConfig("ha-app", "redis-sentinel:26379")
config.Queue.Redis.PoolSize = 100
config.Queue.Redis.MinIdleConns = 20
config.Balancer.Type = load.BalancerTypeRedis
config.Balancer.Strategy = load.StrategyLeastConn
config.Balancer.HeartbeatTTL = 10 * time.Second
```

## Validações Detalhadas

### Validação de Tipos

```go
func validateTypes(config *Config) error {
    // Queue types
    validQueueTypes := map[queue.QueueType]bool{
        queue.QueueTypeMemory: true,
        queue.QueueTypeRedis:  true,
    }
    if !validQueueTypes[config.Queue.Type] {
        return fmt.Errorf("invalid queue type: %s", config.Queue.Type)
    }

    // Balancer types
    validBalancerTypes := map[load.BalancerType]bool{
        load.BalancerTypeMemory: true,
        load.BalancerTypeRedis:  true,
    }
    if !validBalancerTypes[config.Balancer.Type] {
        return fmt.Errorf("invalid balancer type: %s", config.Balancer.Type)
    }

    return nil
}
```

### Validação de Redis

```go
func validateRedisConfig(config *Config) error {
    // Queue Redis validation
    if config.Queue.Type == queue.QueueTypeRedis {
        if config.Queue.Redis == nil {
            return fmt.Errorf("redis config required for redis queue")
        }
        if config.Queue.Redis.Addr == "" {
            return fmt.Errorf("redis address required")
        }
    }

    // Balancer Redis validation
    if config.Balancer.Type == load.BalancerTypeRedis {
        if config.Balancer.Redis == nil {
            return fmt.Errorf("redis config required for redis balancer")
        }
        if config.Balancer.Redis.Addr == "" {
            return fmt.Errorf("redis address required")
        }
    }

    return nil
}
```

### Validação de Estratégias

```go
func validateStrategies(config *Config) error {
    validStrategies := map[load.BalanceStrategy]bool{
        load.StrategyRoundRobin:     true,
        load.StrategyConsistentHash: true,
        load.StrategyWeighted:       true,
        load.StrategyLeastConn:      true,
    }

    if !validStrategies[config.Balancer.Strategy] {
        return fmt.Errorf("invalid balance strategy: %s", config.Balancer.Strategy)
    }

    return nil
}
```

## Configurações Avançadas

### Multi-Redis Setup

```go
// Queue e Balancer em Redis diferentes
config := &config.Config{
    AppName: "multi-redis-app",
    Queue: &config.QueueConfig{
        Type: queue.QueueTypeRedis,
        Redis: &config.RedisConfig{
            Addr: "redis-queue:6379",
            DB:   0,
        },
    },
    Balancer: &config.BalancerConfig{
        Type: load.BalancerTypeRedis,
        Redis: &config.RedisConfig{
            Addr: "redis-balancer:6379",
            DB:   0,
        },
    },
}
```

### Performance Tuning

```go
// Configuração otimizada para alta throughput
config := config.NewRedisConfig("high-perf-app", "redis:6379")

// Queue optimizations
config.Queue.Redis.PoolSize = 100
config.Queue.Redis.MinIdleConns = 25
config.Queue.Redis.MaxRetries = 1  // Fail fast

// Balancer optimizations
config.Balancer.Type = load.BalancerTypeMemory  // Fastest
config.Balancer.Strategy = load.StrategyRoundRobin  // Simplest
config.Balancer.HeartbeatTTL = 60 * time.Second  // Less overhead
config.Balancer.CleanupTicker = 5 * time.Minute  // Less frequent
```

### Security Hardening

```go
config := config.NewRedisConfig("secure-app", "redis:6379")

// Security settings
config.Queue.Redis.Password = os.Getenv("REDIS_PASSWORD")
config.Balancer.Redis.Password = os.Getenv("REDIS_PASSWORD")

// Isolation
config.Queue.Prefix = "secure_queue"
config.Balancer.Prefix = "secure_balancer"

// Conservative timeouts
config.Balancer.HeartbeatTTL = 15 * time.Second
config.Balancer.CleanupTicker = 30 * time.Second
```

## Troubleshooting

### Problemas Comuns

#### Configuração não carrega do ambiente

```go
// Debug: imprimir variáveis
config := config.LoadConfig()
config.Print()

// Verificar se variáveis estão definidas
fmt.Printf("QUEUE_TYPE: %s\n", os.Getenv("QUEUE_TYPE"))
```

#### Validação falha

```go
config := &config.Config{...}
if err := config.Validate(); err != nil {
    log.Printf("Erro de validação: %v", err)
    // Verificar configurações obrigatórias
    // Confirmar tipos e valores válidos
}
```

#### Redis connection issues

```go
// Testar configuração Redis antes de usar
if config.Queue.Type == queue.QueueTypeRedis {
    client := redis.NewClient(&redis.Options{
        Addr: config.Queue.Redis.Addr,
        Password: config.Queue.Redis.Password,
        DB: config.Queue.Redis.DB,
    })

    if err := client.Ping(context.Background()).Err(); err != nil {
        log.Fatalf("Redis não acessível: %v", err)
    }
}
```

## Melhores Práticas

### 1. Use Validação Sempre

```go
config := config.LoadConfig()
if err := config.Validate(); err != nil {
    log.Fatalf("Configuração inválida: %v", err)
}
```

### 2. Configuração por Ambiente

```go
// ✅ Baseado no ambiente
func getConfig() *config.Config {
    if os.Getenv("ENV") == "production" {
        return config.NewRedisConfig("prod-app", redisAddr)
    }
    return config.NewMemoryConfig("dev-app")
}
```

### 3. Não Hardcode Credenciais

```go
// ❌ Evitar
config.Queue.Redis.Password = "senha123"

// ✅ Usar ambiente
config.Queue.Redis.Password = os.Getenv("REDIS_PASSWORD")
```

### 4. Debug com Print

```go
// ✅ Para debug e verificação
config := config.LoadConfig()
config.Print()
```

### 5. Configurações Específicas por Cenário

```go
// ✅ Diferentes configurações para diferentes necessidades
switch useCase {
case "high_throughput":
    config.Queue.Redis.PoolSize = 100
case "low_latency":
    config.Balancer.Type = load.BalancerTypeMemory
case "high_availability":
    config.Balancer.HeartbeatTTL = 10 * time.Second
}
```

## Extensibilidade

### Adicionando Novos Tipos

```go
// Definir novo tipo
type QueueType string
const QueueTypeCustom QueueType = "custom"

// Adicionar à validação
func validateQueueType(qtype QueueType) error {
    valid := map[QueueType]bool{
        QueueTypeMemory: true,
        QueueTypeRedis:  true,
        QueueTypeCustom: true,  // Novo tipo
    }
    // ...
}
```

### Configurações Customizadas

```go
// Estrutura estendida
type CustomConfig struct {
    *config.Config
    CustomField string
    Advanced    *AdvancedOptions
}

// Factory customizada
func NewCustomConfig(appName string) *CustomConfig {
    baseConfig := config.NewMemoryConfig(appName)
    return &CustomConfig{
        Config:      baseConfig,
        CustomField: "default",
        Advanced:    &AdvancedOptions{},
    }
}
```

### Environment Loaders

```go
// Loader customizado
type ConfigLoader interface {
    Load() (*config.Config, error)
}

type EnvLoader struct{}
func (e *EnvLoader) Load() (*config.Config, error) {
    return config.LoadConfig(), nil
}

type FileLoader struct{ path string }
func (f *FileLoader) Load() (*config.Config, error) {
    // Carregar de arquivo JSON/YAML
}
```
