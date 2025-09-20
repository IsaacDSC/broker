# Package Load - Sistema de Balanceamento de Carga

## Visão Geral

O package `load` é responsável pelo balanceamento de carga e distribuição de mensagens entre múltiplos consumidores no sistema de broker. Ele garante que as mensagens sejam distribuídas de forma equitativa e eficiente entre as instâncias ativas, evitando sobrecarga em alguns consumidores enquanto outros ficam ociosos.

## Responsabilidades Principais

- **Registro de Consumidores**: Gerencia o ciclo de vida dos subscribers (registro, heartbeat, remoção)
- **Distribuição de Mensagens**: Implementa diferentes estratégias de balanceamento
- **Monitoramento de Saúde**: Controla subscribers ativos/inativos através de heartbeats
- **Prevenção de Duplicação**: Garante que cada mensagem seja processada por apenas um consumidor
- **Estatísticas**: Coleta e fornece métricas sobre o balanceamento

## Implementações

#### Funções Principais

##### `NewMemoryBalancer(config *BalancerConfig) *MemoryBalancer`

**Responsabilidade**: Cria nova instância do balancer em memória
**Parâmetros**:

- `config`: Configurações do balancer (TTL, estratégia, etc.)
  **Retorno**: Instância configurada do MemoryBalancer
  **Comportamento**:
- Inicializa estruturas de dados thread-safe
- Configura valores padrão se não informados
- Inicia goroutine de limpeza automática

##### `Subscribe(subID uuid.UUID) error`

**Responsabilidade**: Registra um novo subscriber no balancer
**Parâmetros**:

- `subID`: Identificador único do subscriber
  **Retorno**: Error se falhar
  **Comportamento**:
- Adquire lock de escrita
- Cria SubscriberInfo com timestamp atual
- Atualiza estatísticas internas
- Thread-safe através de mutex

##### `Unsubscribe(subID uuid.UUID) error`

**Responsabilidade**: Remove subscriber do balancer
**Parâmetros**:

- `subID`: Identificador do subscriber a remover
  **Retorno**: Error se falhar
  **Comportamento**:
- Marca subscriber como inativo
- Remove da lista de ativos
- Atualiza contadores

##### `ClaimMessage(subID uuid.UUID, messageID string) (bool, error)`

**Responsabilidade**: Determina se um subscriber específico deve processar uma mensagem
**Parâmetros**:

- `subID`: ID do subscriber solicitante
- `messageID`: ID único da mensagem
  **Retorno**:
- `bool`: true se deve processar, false caso contrário
- `error`: erro se houver falha
  **Comportamento**:
- Verifica se subscriber está ativo
- Aplica algoritmo de distribuição baseado na estratégia configurada
- Atualiza contadores de processamento
- Operação atômica para evitar duplicação

##### `UpdateHeartbeat(subID uuid.UUID) error`

**Responsabilidade**: Atualiza timestamp de última atividade do subscriber
**Parâmetros**:

- `subID`: ID do subscriber
  **Retorno**: Error se falhar
  **Comportamento**:
- Atualiza campo LastSeen com timestamp atual
- Essencial para detecção de subscribers inativos

##### `CleanupInactiveSubscribers(timeout time.Duration) error`

**Responsabilidade**: Remove subscribers que não enviaram heartbeat dentro do timeout
**Parâmetros**:

- `timeout`: Tempo limite para considerar subscriber inativo
  **Retorno**: Error se falhar
  **Comportamento**:
- Calcula cutoff time baseado no timeout
- Remove subscribers com LastSeen anterior ao cutoff
- Atualiza estatísticas
- Executado automaticamente via ticker

### Redis Balancer (`redis.go`)

#### Funções Principais

##### `NewRedisBalancer(config *BalancerConfig) (*RedisBalancer, error)`

**Responsabilidade**: Cria instância do balancer Redis
**Parâmetros**:

- `config`: Configurações incluindo conexão Redis
  **Retorno**: Instância configurada ou erro
  **Comportamento**:
- Estabelece conexão com Redis
- Testa conectividade com PING
- Configura prefixos para namespacing
- Inicia cleanup automático

##### `Subscribe(subID uuid.UUID) error`

**Responsabilidade**: Registra subscriber no Redis
**Comportamento**:

- Serializa SubscriberInfo para JSON
- Armazena no Redis com TTL
- Adiciona ao set de subscribers ativos
- Usa pipeline para operações atômicas

##### `ClaimMessage(subID uuid.UUID, messageID string) (bool, error)`

**Responsabilidade**: Implementa claim distribuído de mensagens
**Comportamento**:

- Verifica se subscriber está no set ativo
- Usa SETNX para claim atômico com TTL
- Previne race conditions entre instâncias
- Atualiza estatísticas do subscriber

##### `CleanupInactiveSubscribers(timeout time.Duration) error`

**Responsabilidade**: Remove subscribers inativos usando Lua script
**Comportamento**:

- Executa script Lua para operação atômica
- Itera sobre subscribers verificando LastSeen
- Remove subscribers expirados
- Atualiza estatísticas globais

## Estratégias de Balanceamento

### Round Robin (`StrategyRoundRobin`)

**Algoritmo**: Distribui mensagens sequencialmente
**Implementação**:

```go
func roundRobinStrategy(subID uuid.UUID, activeSubscribers []uuid.UUID) bool {
    index := atomic.AddInt64(&counter, 1) % int64(len(activeSubscribers))
    selectedID := activeSubscribers[index]
    return selectedID == subID
}
```

**Vantagens**: Simples, distribuição uniforme
**Uso**: Quando todos subscribers têm capacidade similar

### Consistent Hash (`StrategyConsistentHash`)

**Algoritmo**: Hash da mensagem determina o subscriber
**Implementação**:

```go
func consistentHashStrategy(subID uuid.UUID, messageKey string, activeSubscribers []uuid.UUID) bool {
    hash := simpleHash(messageKey)
    index := hash % len(activeSubscribers)
    selectedID := activeSubscribers[index]
    return selectedID == subID
}
```

**Vantagens**: Mensagens similares vão para o mesmo subscriber
**Uso**: Quando ordem/localidade é importante

### Weighted (`StrategyWeighted`)

**Algoritmo**: Distribui baseado no peso dos subscribers
**Implementação**:

- Calcula peso total de todos subscribers
- Seleciona baseado na proporção do peso
  **Vantagens**: Permite subscribers com capacidades diferentes
  **Uso**: Quando há heterogeneidade de recursos

### Least Connections (`StrategyLeastConn`)

**Algoritmo**: Direciona para subscriber com menor carga
**Implementação**:

- Compara ProcessCount de todos subscribers
- Seleciona o com menor número
  **Vantagens**: Balanceamento dinâmico baseado em carga real
  **Uso**: Para cargas de trabalho variáveis

## Configuração

### BalancerConfig

```go
type BalancerConfig struct {
    Type          BalancerType        // memory ou redis
    Redis         *RedisConfig        // config Redis se aplicável
    AppName       string              // nome da aplicação
    Strategy      BalanceStrategy     // estratégia de balanceamento
    HeartbeatTTL  time.Duration      // TTL do heartbeat
    CleanupTicker time.Duration      // intervalo de limpeza
    Prefix        string              // prefixo para chaves
}
```

### Variáveis de Ambiente

```bash
BALANCER_TYPE=redis                    # Tipo do balancer
BALANCE_STRATEGY=consistent_hash       # Estratégia
HEARTBEAT_TTL=30s                     # TTL dos heartbeats
CLEANUP_TICKER=60s                    # Intervalo de limpeza
BALANCER_PREFIX=balancer              # Prefixo das chaves
```

## Monitoramento e Métricas

### BalancerStats

```go
type BalancerStats struct {
    TotalSubscribers   int
    ActiveSubscribers  int
    MessagesProcessed  int64
    AverageProcessTime time.Duration
    LastCleanup        time.Time
}
```

### Health Checks

- **Memory**: Sempre saudável (sem dependências externas)
- **Redis**: Verifica conectividade com PING command

## Padrões de Uso

### Subscriber Registration

```go
// Automático via NewSubscribe
subscriber := sub.NewSubscribe(client)
// Balancer.Subscribe() é chamado internamente
```

### Weight Management

```go
// Configurar peso do subscriber
subscriber.SetWeight(3) // Receberá mais mensagens

// Via balancer diretamente
balancer.SetSubscriberWeight(subID, 3)
```

### Monitoring

```go
// Obter estatísticas
stats := balancer.GetStats()
fmt.Printf("Subscribers ativos: %d\n", stats.ActiveSubscribers)

// Health check
if err := balancer.Health(); err != nil {
    log.Printf("Balancer unhealthy: %v", err)
}
```

## Considerações de Performance

### Memory Balancer

- **Prós**: Latência ultra-baixa, sem overhead de rede
- **Contras**: Limitado a single-instance, sem persistência

### Redis Balancer

- **Prós**: Distribuído, persistente, escalável
- **Contras**: Latência de rede, dependência externa

### Otimizações

1. **Pipeline Operations**: Usa Redis pipelines para múltiplas operações
2. **Lua Scripts**: Operações atômicas complexas no servidor
3. **Connection Pooling**: Reutilização de conexões Redis
4. **TTL Management**: Limpeza automática de dados expirados

## Troubleshooting

### Problemas Comuns

#### Subscribers não recebem mensagens

- Verificar se estão registrados: `GetActiveSubscribers()`
- Checar estratégia de balanceamento
- Validar heartbeats estão sendo enviados

#### Distribuição desigual

- Analisar pesos dos subscribers
- Verificar estratégia configurada
- Monitorar ProcessCount individual

#### Redis connection issues

- Verificar conectividade de rede
- Validar credenciais Redis
- Checar configuração de pool

### Debug Tools

```go
// Listar subscribers ativos
active, _ := balancer.GetActiveSubscribers()
fmt.Printf("Active: %v\n", active)

// Informações detalhadas
info, _ := balancer.GetSubscriberInfo(subID)
fmt.Printf("Info: %+v\n", info)
```

## Extensibilidade

### Adicionando Nova Estratégia

1. Definir nova constante em `BalanceStrategy`
2. Implementar função de estratégia
3. Adicionar case em `ShouldProcess()`
4. Documentar comportamento e casos de uso

### Custom Balancer Implementation

1. Implementar interface `Balancer`
2. Registrar na factory se necessário
3. Adicionar configuração apropriada
4. Testes de integração
