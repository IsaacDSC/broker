# Package Queue - Sistema de Filas de Mensagens

## Visão Geral

O package `queue` é responsável pelo armazenamento, gerenciamento e distribuição de mensagens no sistema de broker. Ele fornece abstrações para diferentes tipos de armazenamento (memória e Redis) através de uma interface unificada, garantindo operações thread-safe e controle de duplicação.

## Responsabilidades Principais

- **Armazenamento de Mensagens**: Persiste mensagens com identificadores únicos
- **Controle de Claims**: Implementa sistema de reivindicação atômica de mensagens
- **Prevenção de Duplicação**: Garante que cada mensagem seja processada apenas uma vez
- **Gerenciamento de Ciclo de Vida**: Controla estados das mensagens (nova, claimed, processada)
- **Operações Thread-Safe**: Permite acesso concorrente seguro às filas
- **Abstração de Storage**: Interface unificada para diferentes backends

## Arquitetura

### Interface Principal

```go
type Queue interface {
    Store(key string, value any) error
    Load(key string) (any, bool)
    Delete(key string) error
    ClaimMessage(messageID string) bool
    MarkAsProcessed(messageID string) error
    IsProcessed(messageID string) bool
    GetByMaxConcurrency(maxConcurrency int) Queue
    RangeUnclaimedMessages(fn func(messageID, key string, value any) bool) error
    Range(fn func(key string, value any) bool) error
    Len() (int, error)
    GetUnclaimedMessagesByKey(key string) ([]*Message, error)
    Close() error
    Health() error
}
```

### Estrutura de Dados

#### Message
```go
type Message struct {
    ID      string    // Identificador único da mensagem
    Key     string    // Chave/tipo do evento
    Value   any       // Payload da mensagem
    Claimed bool      // Status de reivindicação
}
```

## Implementações

### Memory Queue (`memory.go`)

#### Características
- Armazenamento em memória RAM
- Performance extremamente alta
- Não persiste dados entre reinicializações
- Ideal para desenvolvimento e testes

#### Estrutura Principal
```go
type MemoryQueue struct {
    messages       map[string]*Message  // Mapa de mensagens ativas
    mutex          sync.RWMutex        // Proteção para mensagens
    processed      map[string]bool     // Registro de mensagens processadas
    processedMutex sync.RWMutex        // Proteção para processadas
    config         *QueueConfig        // Configuração da instância
}
```

#### Funções Principais

##### `NewMemoryQueue(config *QueueConfig) *MemoryQueue`
**Responsabilidade**: Cria nova instância da fila em memória
**Parâmetros**:
- `config`: Configurações da fila (app name, prefixos, etc.)
**Retorno**: Instância configurada do MemoryQueue
**Comportamento**:
- Inicializa mapas thread-safe
- Configura mutexes para proteção
- Armazena configuração para uso posterior

##### `Store(key string, value any) error`
**Responsabilidade**: Armazena nova mensagem na fila
**Parâmetros**:
- `key`: Tipo/chave do evento (ex: "user.created")
- `value`: Payload da mensagem
**Retorno**: Error se falhar
**Comportamento**:
- Gera UUID único para a mensagem
- Cria objeto Message com status não-claimed
- Armazena no mapa protegido por mutex
- Thread-safe através de write lock

##### `Load(key string) (any, bool)`
**Responsabilidade**: Busca primeira mensagem não-claimed de um tipo
**Parâmetros**:
- `key`: Tipo do evento a buscar
**Retorno**:
- `any`: Payload da mensagem encontrada
- `bool`: true se encontrou, false caso contrário
**Comportamento**:
- Itera sobre mensagens com read lock
- Retorna primeira mensagem não-claimed do tipo especificado
- Não remove nem modifica a mensagem

##### `Delete(key string) error`
**Responsabilidade**: Remove primeira mensagem de um tipo específico
**Parâmetros**:
- `key`: Tipo do evento a remover
**Retorno**: Error se falhar
**Comportamento**:
- Busca mensagem do tipo especificado
- Remove do mapa principal
- Operação atômica com write lock

##### `ClaimMessage(messageID string) bool`
**Responsabilidade**: Reivindica mensagem para processamento exclusivo
**Parâmetros**:
- `messageID`: ID único da mensagem
**Retorno**: true se conseguiu claim, false se já claimed
**Comportamento**:
- Verifica se mensagem existe e não está claimed
- Marca como claimed atomicamente
- Previne processamento duplicado
- Operação crítica protegida por mutex

##### `MarkAsProcessed(messageID string) error`
**Responsabilidade**: Marca mensagem como processada e remove da fila
**Parâmetros**:
- `messageID`: ID da mensagem processada
**Retorno**: Error se falhar
**Comportamento**:
- Adiciona ID ao mapa de processadas
- Remove da fila principal
- Usa mutexes separados para melhor performance
- Permite verificação posterior de duplicação

##### `IsProcessed(messageID string) bool`
**Responsabilidade**: Verifica se mensagem já foi processada
**Parâmetros**:
- `messageID`: ID da mensagem a verificar
**Retorno**: true se já processada
**Comportamento**:
- Consulta mapa de processadas com read lock
- Usado para prevenção de reprocessamento

##### `GetUnclaimedMessagesByKey(key string) ([]*Message, error)`
**Responsabilidade**: Retorna todas mensagens não-claimed de um tipo
**Parâmetros**:
- `key`: Tipo do evento
**Retorno**:
- `[]*Message`: Lista de mensagens não-claimed
- `error`: Erro se houver falha
**Comportamento**:
- Filtra mensagens por key e status claimed
- Retorna cópias para evitar modificações externas
- Operação protegida por read lock

##### `RangeUnclaimedMessages(fn func(messageID, key string, value any) bool) error`
**Responsabilidade**: Itera sobre todas mensagens não-claimed
**Parâmetros**:
- `fn`: Função callback para cada mensagem
**Retorno**: Error se falhar
**Comportamento**:
- Cria snapshot das mensagens não-claimed
- Chama função para cada mensagem
- Para se função retornar false
- Thread-safe através de snapshot

### Redis Queue (`redis.go`)

#### Características
- Armazenamento distribuído no Redis
- Persistência entre reinicializações
- Escalabilidade horizontal
- Ideal para produção e ambientes distribuídos

#### Estrutura Principal
```go
type RedisQueue struct {
    client  *redis.Client    // Cliente Redis
    config  *QueueConfig     // Configuração
    ctx     context.Context  // Context para operações
    prefix  string          // Prefixo para namespacing
    lockTTL time.Duration   // TTL para locks
}
```

#### Funções Principais

##### `NewRedisQueue(config *QueueConfig) (*RedisQueue, error)`
**Responsabilidade**: Cria instância da fila Redis
**Parâmetros**:
- `config`: Configurações incluindo conexão Redis
**Retorno**: Instância configurada ou erro
**Comportamento**:
- Valida configuração Redis obrigatória
- Configura pool de conexões com defaults
- Testa conectividade com PING
- Configura prefixos para namespacing

##### `Store(key string, value any) error`
**Responsabilidade**: Armazena mensagem no Redis
**Comportamento**:
- Gera UUID único para messageID
- Serializa Message para JSON
- Usa pipeline para operações atômicas:
  - Armazena mensagem serializada
  - Adiciona ID à lista do evento
- Garante consistência através de transação

##### `ClaimMessage(messageID string) bool`
**Responsabilidade**: Implementa claim distribuído com Redis
**Comportamento**:
- Usa SETNX para claim atômico
- Define TTL para evitar locks órfãos
- Operação atômica previne race conditions
- Retorna true apenas se conseguiu o lock

##### `MarkAsProcessed(messageID string) error`
**Responsabilidade**: Marca como processada e limpa dados
**Comportamento**:
- Usa pipeline Redis para operações atômicas:
  - Marca como processada com TTL
  - Remove mensagem original
  - Remove claim lock
  - Remove de listas de eventos via Lua script
- Script Lua garante atomicidade no servidor

##### `GetUnclaimedMessagesByKey(key string) ([]*Message, error)`
**Responsabilidade**: Busca mensagens não-claimed de um tipo
**Comportamento**:
- Busca IDs da lista do evento no Redis
- Para cada ID, verifica se não está claimed
- Verifica se não foi processada
- Deserializa e retorna mensagens válidas
- Filtragem distribuída entre múltiplas instâncias

##### `CleanupExpiredClaims() error`
**Responsabilidade**: Remove claims expirados (manutenção)
**Comportamento**:
- Busca todas chaves de claims
- Verifica timestamps versus TTL
- Remove claims expirados em batch
- Permite reprocessamento de mensagens órfãs

#### Estratégias de Chaves Redis

##### Padrões de Nomenclatura
```go
// Mensagem individual
fmt.Sprintf("%s:messages:%s", prefix, messageID)

// Lista de eventos
fmt.Sprintf("%s:events:%s", prefix, eventKey)

// Mensagem processada
fmt.Sprintf("%s:processed:%s", prefix, messageID)

// Claim de mensagem
fmt.Sprintf("%s:claims:%s", prefix, messageID)
```

##### Estruturas de Dados Redis
- **Hash**: Para mensagens individuais (serialização JSON)
- **List**: Para filas de eventos (FIFO)
- **String**: Para claims e flags de processamento
- **Sets**: Para operações de limpeza e lookup

## Configuração

### QueueConfig
```go
type QueueConfig struct {
    Type    QueueType     // memory ou redis
    Redis   *RedisConfig  // configuração Redis
    AppName string        // nome da aplicação
    Prefix  string        // prefixo para chaves
}
```

### RedisConfig
```go
type RedisConfig struct {
    Addr         string  // endereço Redis
    Password     string  // senha (opcional)
    DB           int     // banco de dados
    PoolSize     int     // tamanho do pool
    MaxRetries   int     // tentativas máximas
    MinIdleConns int     // conexões mínimas idle
}
```

### Variáveis de Ambiente
```bash
QUEUE_TYPE=redis                    # Tipo da fila
QUEUE_PREFIX=queue                  # Prefixo das chaves
QUEUE_REDIS_ADDR=localhost:6379    # Endereço Redis
QUEUE_REDIS_PASSWORD=secret        # Senha Redis
QUEUE_REDIS_DB=0                   # Banco Redis
QUEUE_REDIS_POOL_SIZE=10           # Tamanho do pool
```

## Padrões de Uso

### Operações Básicas
```go
// Armazenar mensagem
err := queue.Store("user.created", userData)

// Buscar mensagem
value, found := queue.Load("user.created")

// Claim para processamento
claimed := queue.ClaimMessage(messageID)

// Marcar como processada
err := queue.MarkAsProcessed(messageID)
```

### Processamento em Lote
```go
// Buscar mensagens por tipo
messages, err := queue.GetUnclaimedMessagesByKey("user.created")

for _, msg := range messages {
    if queue.ClaimMessage(msg.ID) {
        // Processar mensagem
        processMessage(msg.Value)
        queue.MarkAsProcessed(msg.ID)
    }
}
```

### Iteração Segura
```go
// Iterar sobre todas mensagens
queue.RangeUnclaimedMessages(func(messageID, key string, value any) bool {
    if queue.ClaimMessage(messageID) {
        // Processar
        queue.MarkAsProcessed(messageID)
    }
    return true // continuar iteração
})
```

## Estados da Mensagem

### Ciclo de Vida
1. **Nova**: Criada via `Store()`, disponível para claim
2. **Claimed**: Reivindicada via `ClaimMessage()`, sendo processada
3. **Processada**: Marcada via `MarkAsProcessed()`, removida da fila
4. **Expirada**: Claim TTL expirou, volta para Nova (Redis only)

### Diagramas de Estado
```
Nova → [ClaimMessage] → Claimed → [MarkAsProcessed] → Processada
  ↑                        ↓
  ←────── [TTL Expired] ────┘ (Redis only)
```

## Considerações de Performance

### Memory Queue
- **Prós**:
  - Latência ultra-baixa (nanossegundos)
  - Sem overhead de serialização
  - Sem dependências externas
- **Contras**:
  - Limitada à memória disponível
  - Não persiste entre reinicializações
  - Não compartilhada entre instâncias

### Redis Queue
- **Prós**:
  - Persistência garantida
  - Compartilhamento entre instâncias
  - Escalabilidade horizontal
  - TTL automático para cleanup
- **Contras**:
  - Latência de rede (milissegundos)
  - Overhead de serialização JSON
  - Dependência externa (Redis)

### Otimizações

#### Memory Queue
1. **Separate Mutexes**: Mutexes separados para diferentes operações
2. **Read/Write Locks**: RWMutex para melhor concorrência de leitura
3. **Map Pre-allocation**: Mapas com capacidade inicial quando possível

#### Redis Queue
1. **Pipelining**: Múltiplas operações em uma transação
2. **Lua Scripts**: Operações atômicas complexas no servidor
3. **Connection Pooling**: Reutilização de conexões
4. **JSON Marshaling**: Serialização eficiente de dados

## Monitoring e Observabilidade

### Métricas Importantes
- **Queue Length**: Número de mensagens pendentes
- **Processing Rate**: Mensagens processadas por segundo
- **Claim Success Rate**: Porcentagem de claims bem-sucedidos
- **Average Processing Time**: Tempo médio de processamento

### Health Checks
```go
// Verificar saúde da fila
if err := queue.Health(); err != nil {
    log.Printf("Queue unhealthy: %v", err)
}

// Estatísticas
length, _ := queue.Len()
log.Printf("Queue length: %d", length)
```

### Debugging
```go
// Listar mensagens pendentes
messages, _ := queue.GetUnclaimedMessagesByKey("event_type")
for _, msg := range messages {
    log.Printf("Pending: %s = %v", msg.ID, msg.Value)
}
```

## Troubleshooting

### Problemas Comuns

#### Mensagens não são processadas
- Verificar se claims estão funcionando
- Checar se MarkAsProcessed está sendo chamado
- Validar configuração de TTL (Redis)

#### Memory leaks
- Garantir que MarkAsProcessed é sempre chamado
- Implementar limpeza periódica de processadas antigas
- Monitorar tamanho dos mapas internos

#### Redis connection issues
- Verificar conectividade de rede
- Validar configurações de pool
- Monitorar timeouts e retries

#### Performance degradation
- Analisar tamanho das filas
- Verificar taxa de claim/processamento
- Monitorar uso de memória/CPU

### Debug Tools
```go
// Estatísticas detalhadas
length, _ := queue.Len()
fmt.Printf("Queue length: %d\n", length)

// Verificar mensagem específica
isProcessed := queue.IsProcessed(messageID)
fmt.Printf("Processed: %v\n", isProcessed)

// Iterar para debug
queue.RangeUnclaimedMessages(func(id, key string, value any) bool {
    fmt.Printf("Message: %s, Key: %s, Value: %v\n", id, key, value)
    return true
})
```

## Extensibilidade

### Adicionando Nova Implementação
1. Implementar interface `Queue`
2. Criar factory method appropriado
3. Adicionar testes de integração
4. Documentar características específicas

### Custom Message Types
1. Definir estrutura custom que implementa `any`
2. Garantir que é serializável (JSON para Redis)
3. Implementar validação se necessário
4. Documentar formato esperado

### Performance Tuning
1. Benchmarking com diferentes configurações
2. Profiling de operações críticas
3. Otimização de hot paths
4. Tuning de parâmetros Redis
