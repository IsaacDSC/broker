# Package Factory - Sistema de Criação de Componentes

## Visão Geral

O package `factory` implementa o padrão Factory para criação de componentes do sistema broker. Ele centraliza a lógica de instanciação de Queue e Balancer baseado em configurações, fornecendo uma interface consistente para criação de diferentes implementações (memória, Redis) e garantindo que os componentes sejam criados corretamente com suas dependências.

## Responsabilidades Principais

- **Abstração de Criação**: Encapsula lógica complexa de instanciação de componentes
- **Configuração Centralizada**: Aplica configurações específicas na criação dos objetos
- **Validação de Dependências**: Garante que componentes tenham dependências corretas
- **Health Management**: Fornece ferramentas para verificação de saúde dos componentes
- **Lifecycle Management**: Gerencia criação, validação e encerramento de componentes
- **Error Handling**: Trata erros de criação de forma consistente

## Arquitetura

### Interface Principal

```go
type Factory interface {
    CreateQueue(config *config.Config) (queue.Queue, error)
    CreateBalancer(config *config.Config) (load.Balancer, error)
}
```

### Implementação Padrão

```go
type DefaultFactory struct{}
```

### Factories Especializadas

```go
type QueueFactory struct{}
type BalancerFactory struct{}
type BrokerComponentsFactory struct {
    config *config.Config
}
```

## Funções de Criação Principal

### `NewFactory() Factory`

**Responsabilidade**: Cria instância da factory padrão
**Retorno**: Implementação da interface Factory
**Comportamento**:
- Retorna DefaultFactory configurada
- Ponto de entrada principal para criação de componentes
- Thread-safe por design

**Exemplo**:
```go
factory := factory.NewFactory()
queue, err := factory.CreateQueue(config)
balancer, err := factory.CreateBalancer(config)
```

### `(f *DefaultFactory) CreateQueue(config *config.Config) (queue.Queue, error)`

**Responsabilidade**: Cria instância de Queue baseada na configuração
**Parâmetros**:
- `config`: Configuração completa do sistema
**Retorno**: Interface queue.Queue ou erro
**Comportamento**:
- Converte configuração para formato específico do queue
- Switch baseado no tipo configurado (memory/redis)
- Valida dependências necessárias
- Retorna instância configurada

**Tipos Suportados**:
- `queue.QueueTypeMemory`: Cria MemoryQueue
- `queue.QueueTypeRedis`: Cria RedisQueue com validação de conexão

**Implementação**:
```go
func (f *DefaultFactory) CreateQueue(config *config.Config) (queue.Queue, error) {
    queueConfig := config.Queue.ToQueueConfig(config.AppName)

    switch queueConfig.Type {
    case queue.QueueTypeMemory:
        return queue.NewMemoryQueue(queueConfig), nil
    case queue.QueueTypeRedis:
        return queue.NewRedisQueue(queueConfig)
    default:
        return nil, fmt.Errorf("unsupported queue type: %s", queueConfig.Type)
    }
}
```

### `(f *DefaultFactory) CreateBalancer(config *config.Config) (load.Balancer, error)`

**Responsabilidade**: Cria instância de Balancer baseada na configuração
**Parâmetros**:
- `config`: Configuração completa do sistema
**Retorno**: Interface load.Balancer ou erro
**Comportamento**:
- Converte configuração para formato específico do load
- Switch baseado no tipo configurado (memory/redis)
- Aplica estratégia de balanceamento configurada
- Configura TTLs e intervalos de cleanup

**Tipos Suportados**:
- `load.BalancerTypeMemory`: Cria MemoryBalancer
- `load.BalancerTypeRedis`: Cria RedisBalancer com validação de conexão

**Estratégias Aplicadas**:
- Round Robin, Consistent Hash, Weighted, Least Connections
- TTL de heartbeat e intervalos de cleanup
- Configurações específicas de Redis se aplicável

## Factories Especializadas

### QueueFactory

#### `NewQueueFactory() *QueueFactory`

**Responsabilidade**: Cria factory específica para Queue
**Uso**: Quando você precisa criar apenas filas

#### `(qf *QueueFactory) Create(config *config.Config) (queue.Queue, error)`

**Responsabilidade**: Wrapper para criação de Queue
**Comportamento**: Delega para DefaultFactory.CreateQueue()

#### `(qf *QueueFactory) CreateMemoryQueue(appName string) queue.Queue`

**Responsabilidade**: Cria Queue em memória com configuração mínima
**Parâmetros**:
- `appName`: Nome da aplicação
**Retorno**: Queue em memória configurada
**Comportamento**:
- Configuração padrão para desenvolvimento
- Sem validação de erro (sempre sucesso)
- Configurações otimizadas para performance

**Exemplo**:
```go
queueFactory := factory.NewQueueFactory()
queue := queueFactory.CreateMemoryQueue("dev-app")
```

#### `(qf *QueueFactory) CreateRedisQueue(appName, redisAddr string) (queue.Queue, error)`

**Responsabilidade**: Cria Queue Redis com configuração padrão
**Parâmetros**:
- `appName`: Nome da aplicação
- `redisAddr`: Endereço do servidor Redis
**Retorno**: Queue Redis configurada ou erro
**Comportamento**:
- Configurações padrão de produção
- Validação de conectividade
- Pool de conexões configurado

**Configurações Aplicadas**:
```go
Redis: &queue.RedisConfig{
    Addr:         redisAddr,
    Password:     "",
    DB:           0,
    PoolSize:     10,
    MaxRetries:   3,
    MinIdleConns: 2,
}
```

### BalancerFactory

#### `NewBalancerFactory() *BalancerFactory`

**Responsabilidade**: Cria factory específica para Balancer
**Uso**: Quando você precisa criar apenas balanceadores

#### `(bf *BalancerFactory) CreateMemoryBalancer(appName string, strategy load.BalanceStrategy) load.Balancer`

**Responsabilidade**: Cria Balancer em memória com estratégia específica
**Parâmetros**:
- `appName`: Nome da aplicação
- `strategy`: Estratégia de balanceamento desejada
**Retorno**: Balancer em memória configurado
**Características**:
- Configuração otimizada para performance
- Estratégia customizável
- TTLs padrão configurados

#### `(bf *BalancerFactory) CreateRedisBalancer(appName, redisAddr string, strategy load.BalanceStrategy) (load.Balancer, error)`

**Responsabilidade**: Cria Balancer Redis com estratégia específica
**Parâmetros**:
- `appName`: Nome da aplicação
- `redisAddr`: Endereço do Redis
- `strategy`: Estratégia de balanceamento
**Retorno**: Balancer Redis configurado ou erro
**Comportamento**:
- Validação de conectividade Redis
- Configuração de TTLs distribuídos
- Pool de conexões otimizado

## BrokerComponentsFactory

### `NewBrokerComponentsFactory(config *config.Config) *BrokerComponentsFactory`

**Responsabilidade**: Cria factory para criação coordenada de todos os componentes
**Parâmetros**:
- `config`: Configuração completa validada
**Retorno**: Factory configurada para o broker completo
**Uso**: Criação de cliente broker completo

### `(bcf *BrokerComponentsFactory) CreateComponents() (queue.Queue, load.Balancer, error)`

**Responsabilidade**: Cria Queue e Balancer coordenadamente
**Retorno**: Queue, Balancer configurados ou erro
**Comportamento**:
- Cria Queue primeiro
- Se Queue falhar, retorna erro imediatamente
- Cria Balancer
- Se Balancer falhar, fecha Queue antes de retornar erro
- Garante que componentes são criados em ordem correta

**Algoritmo**:
```go
func (bcf *BrokerComponentsFactory) CreateComponents() (queue.Queue, load.Balancer, error) {
    factory := NewFactory()

    // Cria Queue
    q, err := factory.CreateQueue(bcf.config)
    if err != nil {
        return nil, nil, fmt.Errorf("failed to create queue: %w", err)
    }

    // Cria Balancer
    b, err := factory.CreateBalancer(bcf.config)
    if err != nil {
        q.Close() // Cleanup em caso de erro
        return nil, nil, fmt.Errorf("failed to create balancer: %w", err)
    }

    return q, b, nil
}
```

### `(bcf *BrokerComponentsFactory) ValidateAndCreate() (queue.Queue, load.Balancer, error)`

**Responsabilidade**: Valida configuração antes de criar componentes
**Retorno**: Componentes validados e criados ou erro
**Comportamento**:
- Executa bcf.config.Validate() primeiro
- Se validação falhar, retorna erro sem criar componentes
- Se validação passar, delega para CreateComponents()
- Garante que apenas configurações válidas sejam usadas

## Funções Utilitárias

### `ComponentsHealth(q queue.Queue, b load.Balancer) error`

**Responsabilidade**: Verifica saúde de componentes criados
**Parâmetros**:
- `q`: Interface Queue para verificar
- `b`: Interface Balancer para verificar
**Retorno**: Error se algum componente não está saudável
**Comportamento**:
- Chama q.Health() para verificar Queue
- Chama b.Health() para verificar Balancer
- Retorna erro agregado se algum falhar
- Útil para health checks pós-criação

**Exemplo**:
```go
q, b, err := factory.CreateComponents()
if err != nil {
    log.Fatal(err)
}

if err := factory.ComponentsHealth(q, b); err != nil {
    log.Printf("Componentes não saudáveis: %v", err)
}
```

### `CloseComponents(q queue.Queue, b load.Balancer) error`

**Responsabilidade**: Fecha componentes de forma segura
**Parâmetros**:
- `q`: Queue para fechar (pode ser nil)
- `b`: Balancer para fechar (pode ser nil)
**Retorno**: Error agregado se algum close falhar
**Comportamento**:
- Tenta fechar Queue se não for nil
- Tenta fechar Balancer se não for nil
- Coleta todos os erros
- Retorna erro agregado ou nil se todos fecharam com sucesso
- Não falha se componente for nil (defensive programming)

**Algoritmo**:
```go
func CloseComponents(q queue.Queue, b load.Balancer) error {
    var errors []error

    if q != nil {
        if err := q.Close(); err != nil {
            errors = append(errors, fmt.Errorf("failed to close queue: %w", err))
        }
    }

    if b != nil {
        if err := b.Close(); err != nil {
            errors = append(errors, fmt.Errorf("failed to close balancer: %w", err))
        }
    }

    if len(errors) > 0 {
        return fmt.Errorf("errors closing components: %v", errors)
    }

    return nil
}
```

## Padrões de Uso

### Criação Simples de Componentes

```go
// Factory padrão
factory := factory.NewFactory()

// Criar Queue
queue, err := factory.CreateQueue(config)
if err != nil {
    log.Fatal(err)
}

// Criar Balancer
balancer, err := factory.CreateBalancer(config)
if err != nil {
    log.Fatal(err)
}
```

### Criação Especializada

```go
// Queue factory
qf := factory.NewQueueFactory()
queue := qf.CreateMemoryQueue("dev-app")

// Balancer factory
bf := factory.NewBalancerFactory()
balancer := bf.CreateMemoryBalancer("dev-app", load.StrategyRoundRobin)
```

### Criação Coordenada (Broker Completo)

```go
// Validar configuração
if err := config.Validate(); err != nil {
    log.Fatal(err)
}

// Factory coordenada
factory := factory.NewBrokerComponentsFactory(config)

// Criar todos os componentes
queue, balancer, err := factory.ValidateAndCreate()
if err != nil {
    log.Fatal(err)
}

// Verificar saúde
if err := factory.ComponentsHealth(queue, balancer); err != nil {
    log.Printf("Componentes não saudáveis: %v", err)
}

// Usar componentes...

// Cleanup
defer factory.CloseComponents(queue, balancer)
```

### Criação com Error Handling Robusto

```go
func createBrokerComponents(config *config.Config) (queue.Queue, load.Balancer, error) {
    // Validar primeiro
    if err := config.Validate(); err != nil {
        return nil, nil, fmt.Errorf("configuração inválida: %w", err)
    }

    // Factory
    factory := factory.NewBrokerComponentsFactory(config)

    // Criar componentes
    q, b, err := factory.CreateComponents()
    if err != nil {
        return nil, nil, fmt.Errorf("falha na criação: %w", err)
    }

    // Health check
    if err := factory.ComponentsHealth(q, b); err != nil {
        // Cleanup se health check falhar
        factory.CloseComponents(q, b)
        return nil, nil, fmt.Errorf("componentes não saudáveis: %w", err)
    }

    return q, b, nil
}
```

## Error Handling

### Tipos de Erros

#### Configuração Inválida
```go
// Tipo não suportado
return nil, fmt.Errorf("unsupported queue type: %s", queueConfig.Type)

// Configuração obrigatória ausente
return nil, fmt.Errorf("redis config is required")
```

#### Falha de Conectividade
```go
// Redis inacessível
return nil, fmt.Errorf("failed to connect to redis: %w", err)

// Health check falhou
return fmt.Errorf("queue health check failed: %w", err)
```

#### Falha de Criação
```go
// Factory falhou
return nil, nil, fmt.Errorf("failed to create components: %w", err)

// Cleanup necessário
q.Close()
return nil, nil, fmt.Errorf("failed to create balancer: %w", err)
```

### Estratégias de Recovery

#### Retry com Backoff
```go
func createWithRetry(config *config.Config, maxRetries int) (queue.Queue, load.Balancer, error) {
    factory := factory.NewBrokerComponentsFactory(config)

    for i := 0; i < maxRetries; i++ {
        q, b, err := factory.CreateComponents()
        if err == nil {
            return q, b, nil
        }

        log.Printf("Tentativa %d falhou: %v", i+1, err)
        time.Sleep(time.Duration(i+1) * time.Second)
    }

    return nil, nil, fmt.Errorf("falhou após %d tentativas", maxRetries)
}
```

#### Fallback para Memory
```go
func createWithFallback(config *config.Config) (queue.Queue, load.Balancer, error) {
    factory := factory.NewBrokerComponentsFactory(config)

    // Tentar configuração original
    q, b, err := factory.CreateComponents()
    if err == nil {
        return q, b, nil
    }

    log.Printf("Fallback para memória devido a: %v", err)

    // Fallback para memory
    memConfig := config.NewMemoryConfig(config.AppName)
    memFactory := factory.NewBrokerComponentsFactory(memConfig)

    return memFactory.CreateComponents()
}
```

## Performance e Otimizações

### Lazy Initialization

```go
type LazyFactory struct {
    config *config.Config
    queue  queue.Queue
    balancer load.Balancer
    mutex  sync.Mutex
}

func (lf *LazyFactory) GetQueue() queue.Queue {
    lf.mutex.Lock()
    defer lf.mutex.Unlock()

    if lf.queue == nil {
        factory := factory.NewFactory()
        lf.queue, _ = factory.CreateQueue(lf.config)
    }

    return lf.queue
}
```

### Component Pooling

```go
type ComponentPool struct {
    queues    []queue.Queue
    balancers []load.Balancer
    config    *config.Config
    mutex     sync.Mutex
}

func (cp *ComponentPool) GetQueue() queue.Queue {
    cp.mutex.Lock()
    defer cp.mutex.Unlock()

    if len(cp.queues) > 0 {
        q := cp.queues[len(cp.queues)-1]
        cp.queues = cp.queues[:len(cp.queues)-1]
        return q
    }

    // Criar novo se pool vazio
    factory := factory.NewFactory()
    q, _ := factory.CreateQueue(cp.config)
    return q
}
```

### Caching de Factories

```go
var factoryCache = make(map[string]Factory)
var cacheMutex sync.RWMutex

func GetCachedFactory(configHash string) Factory {
    cacheMutex.RLock()
    if f, exists := factoryCache[configHash]; exists {
        cacheMutex.RUnlock()
        return f
    }
    cacheMutex.RUnlock()

    cacheMutex.Lock()
    defer cacheMutex.Unlock()

    // Double-check locking
    if f, exists := factoryCache[configHash]; exists {
        return f
    }

    f := factory.NewFactory()
    factoryCache[configHash] = f
    return f
}
```

## Testing

### Mock Factory

```go
type MockFactory struct {
    QueueResult    queue.Queue
    BalancerResult load.Balancer
    QueueError     error
    BalancerError  error
}

func (mf *MockFactory) CreateQueue(config *config.Config) (queue.Queue, error) {
    return mf.QueueResult, mf.QueueError
}

func (mf *MockFactory) CreateBalancer(config *config.Config) (load.Balancer, error) {
    return mf.BalancerResult, mf.BalancerError
}
```

### Integration Tests

```go
func TestFactoryIntegration(t *testing.T) {
    config := config.NewMemoryConfig("test-app")

    factory := factory.NewBrokerComponentsFactory(config)
    q, b, err := factory.ValidateAndCreate()

    assert.NoError(t, err)
    assert.NotNil(t, q)
    assert.NotNil(t, b)

    // Health check
    err = factory.ComponentsHealth(q, b)
    assert.NoError(t, err)

    // Cleanup
    err = factory.CloseComponents(q, b)
    assert.NoError(t, err)
}
```

## Melhores Práticas

### 1. Sempre Valide Configurações
```go
// ✅ Validar antes de criar
if err := config.Validate(); err != nil {
    return nil, nil, err
}
```

### 2. Use Cleanup Apropriado
```go
// ✅ Cleanup em caso de erro parcial
q, err := factory.CreateQueue(config)
if err != nil {
    return nil, nil, err
}

b, err := factory.CreateBalancer(config)
if err != nil {
    q.Close() // ✅ Cleanup da queue
    return nil, nil, err
}
```

### 3. Health Checks Pós-Criação
```go
// ✅ Verificar saúde após criação
if err := factory.ComponentsHealth(q, b); err != nil {
    factory.CloseComponents(q, b)
    return nil, nil, err
}
```

### 4. Error Wrapping
```go
// ✅ Contexto nos erros
if err != nil {
    return nil, nil, fmt.Errorf("failed to create queue: %w", err)
}
```

### 5. Defer Cleanup
```go
// ✅ Sempre usar defer
defer factory.CloseComponents(queue, balancer)
```

## Extensibilidade

### Adicionando Novo Tipo de Factory

```go
type CustomFactory struct {
    defaultFactory *DefaultFactory
    customOptions  map[string]interface{}
}

func (cf *CustomFactory) CreateQueue(config *config.Config) (queue.Queue, error) {
    // Lógica customizada
    if customType, ok := cf.customOptions["custom_queue"]; ok {
        return cf.createCustomQueue(customType, config)
    }

    // Fallback para padrão
    return cf.defaultFactory.CreateQueue(config)
}
```

### Factory Registry

```go
type FactoryRegistry struct {
    factories map[string]Factory
    mutex     sync.RWMutex
}

func (fr *FactoryRegistry) Register(name string, factory Factory) {
    fr.mutex.Lock()
    defer fr.mutex.Unlock()
    fr.factories[name] = factory
}

func (fr *FactoryRegistry) Get(name string) (Factory, bool) {
    fr.mutex.RLock()
    defer fr.mutex.RUnlock()
    f, exists := fr.factories[name]
    return f, exists
}
```

### Builder Pattern Integration

```go
type ComponentBuilder struct {
    config  *config.Config
    factory Factory
}

func NewComponentBuilder() *ComponentBuilder {
    return &ComponentBuilder{
        factory: factory.NewFactory(),
    }
}

func (cb *ComponentBuilder) WithConfig(config *config.Config) *ComponentBuilder {
    cb.config = config
    return cb
}

func (cb *ComponentBuilder) WithCustomFactory(factory Factory) *ComponentBuilder {
    cb.factory = factory
    return cb
}

func (cb *ComponentBuilder) Build() (queue.Queue, load.Balancer, error) {
    if cb.config == nil {
        return nil, nil, fmt.Errorf("config is required")
    }

    q, err := cb.factory.CreateQueue(cb.config)
    if err != nil {
        return nil, nil, err
    }

    b, err := cb.factory.CreateBalancer(cb.config)
    if err != nil {
        q.Close()
        return nil, nil, err
    }

    return q, b, nil
}
```
