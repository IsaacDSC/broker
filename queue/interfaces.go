package queue

import "github.com/google/uuid"

// Message representa uma mensagem na fila
type Message struct {
	ID      string
	Key     string
	Value   any
	Claimed bool
}

// Queue interface define as operações básicas de uma fila
type Queue interface {
	// Store adiciona uma nova mensagem à fila
	Store(key string, value any) error

	// Load busca uma mensagem por chave
	Load(key string) (any, bool)

	// Delete remove uma mensagem da fila
	Delete(key string) error

	// ClaimMessage tenta reivindicar uma mensagem para processamento
	ClaimMessage(messageID string) bool

	// MarkAsProcessed marca uma mensagem como processada
	MarkAsProcessed(messageID string) error

	// IsProcessed verifica se uma mensagem já foi processada
	IsProcessed(messageID string) bool

	// GetByMaxConcurrency retorna uma nova fila com no máximo maxConcurrency mensagens
	GetByMaxConcurrency(maxConcurrency int) Queue

	// RangeUnclaimedMessages itera sobre mensagens não reivindicadas
	RangeUnclaimedMessages(fn func(messageID, key string, value any) bool) error

	// Range itera sobre todas as mensagens não reivindicadas
	Range(fn func(key string, value any) bool) error

	// Len retorna o número de mensagens não reivindicadas
	Len() (int, error)

	// GetUnclaimedMessagesByKey retorna mensagens não reivindicadas por chave específica
	GetUnclaimedMessagesByKey(key string) ([]*Message, error)

	// Close fecha a conexão da fila (se aplicável)
	Close() error

	// Health verifica se a fila está saudável
	Health() error
}

// QueueConfig configurações para criação da fila
type QueueConfig struct {
	Type    QueueType
	Redis   *RedisConfig
	AppName string
	Prefix  string
}

// QueueType tipo de implementação da fila
type QueueType string

const (
	QueueTypeMemory QueueType = "memory"
	QueueTypeRedis  QueueType = "redis"
)

// RedisConfig configurações específicas do Redis
type RedisConfig struct {
	Addr         string
	Password     string
	DB           int
	PoolSize     int
	MaxRetries   int
	MinIdleConns int
}

// QueueFactory interface para criar filas
type QueueFactory interface {
	CreateQueue(config *QueueConfig) (Queue, error)
}

// SubscriberInfo informações do subscriber para o balancer
type SubscriberInfo struct {
	ID       uuid.UUID
	Active   bool
	LastSeen int64
}

// MessageLock representa um lock de mensagem
type MessageLock struct {
	MessageID    string
	SubscriberID uuid.UUID
	LockTime     int64
	TTL          int64
}
