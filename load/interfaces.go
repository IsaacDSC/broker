package load

import (
	"time"

	"github.com/google/uuid"
)

// Balancer interface define as operações de balanceamento de carga
type Balancer interface {
	// Subscribe registra um novo subscriber
	Subscribe(subID uuid.UUID) error

	// Unsubscribe remove um subscriber
	Unsubscribe(subID uuid.UUID) error

	// GetTotalSubscribers retorna o número total de subscribers ativos
	GetTotalSubscribers() (int, error)

	// ShouldProcess determina se este subscriber deve processar a mensagem
	ShouldProcess(subID uuid.UUID, messageKey string) (bool, error)

	// ClaimMessage tenta reivindicar uma mensagem para processamento
	ClaimMessage(subID uuid.UUID, messageID string) (bool, error)

	// GetActiveSubscribers retorna lista de subscribers ativos
	GetActiveSubscribers() ([]uuid.UUID, error)

	// UpdateHeartbeat atualiza o heartbeat de um subscriber
	UpdateHeartbeat(subID uuid.UUID) error

	// CleanupInactiveSubscribers remove subscribers inativos
	CleanupInactiveSubscribers(timeout time.Duration) error

	// GetSubscriberInfo retorna informações de um subscriber
	GetSubscriberInfo(subID uuid.UUID) (*SubscriberInfo, error)

	// SetSubscriberWeight define o peso de um subscriber para balanceamento
	SetSubscriberWeight(subID uuid.UUID, weight int) error

	// Close fecha a conexão do balancer (se aplicável)
	Close() error

	// Health verifica se o balancer está saudável
	Health() error
}

// BalancerConfig configurações para criação do balancer
type BalancerConfig struct {
	Type          BalancerType
	Redis         *RedisConfig
	AppName       string
	Prefix        string
	HeartbeatTTL  time.Duration
	CleanupTicker time.Duration
	Strategy      BalanceStrategy
}

// BalancerType tipo de implementação do balancer
type BalancerType string

const (
	BalancerTypeMemory BalancerType = "memory"
	BalancerTypeRedis  BalancerType = "redis"
)

// BalanceStrategy estratégia de balanceamento
type BalanceStrategy string

const (
	StrategyRoundRobin     BalanceStrategy = "round_robin"
	StrategyConsistentHash BalanceStrategy = "consistent_hash"
	StrategyWeighted       BalanceStrategy = "weighted"
	StrategyLeastConn      BalanceStrategy = "least_conn"
)

// RedisConfig configurações específicas do Redis para balancer
type RedisConfig struct {
	Addr         string
	Password     string
	DB           int
	PoolSize     int
	MaxRetries   int
	MinIdleConns int
}

// SubscriberInfo informações do subscriber
type SubscriberInfo struct {
	ID           uuid.UUID
	Active       bool
	LastSeen     int64
	Weight       int
	ProcessCount int64
	JoinedAt     int64
}

// BalancerFactory interface para criar balancers
type BalancerFactory interface {
	CreateBalancer(config *BalancerConfig) (Balancer, error)
}

// BalancerStats estatísticas do balancer
type BalancerStats struct {
	TotalSubscribers   int
	ActiveSubscribers  int
	MessagesProcessed  int64
	AverageProcessTime time.Duration
	LastCleanup        time.Time
}

// MessageDistribution informações sobre distribuição de mensagem
type MessageDistribution struct {
	MessageID    string
	SubscriberID uuid.UUID
	Strategy     BalanceStrategy
	Timestamp    time.Time
	Weight       int
}
