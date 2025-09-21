package broker

import (
	"fmt"

	"github.com/IsaacDSC/broker/config"
	"github.com/IsaacDSC/broker/factory"
	"github.com/IsaacDSC/broker/load"
	"github.com/IsaacDSC/broker/queue"
	"github.com/google/uuid"
)

type Client struct {
	ID       uuid.UUID
	appName  string
	config   *config.Config
	Queue    queue.Queue
	Balancer load.Balancer
}

// NewClient cria um novo cliente com configuração padrão (memória)
func NewClient(appName string) (*Client, error) {
	config := config.NewMemoryConfig(appName)
	return NewClientWithConfig(config)
}

// NewClientWithConfig cria um novo cliente com configuração específica
func NewClientWithConfig(cfg *config.Config) (*Client, error) {
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	factory := factory.NewBrokerComponentsFactory(cfg)
	q, b, err := factory.ValidateAndCreate()
	if err != nil {
		return nil, fmt.Errorf("failed to create components: %w", err)
	}

	return &Client{
		ID:       uuid.New(),
		appName:  cfg.AppName,
		config:   cfg,
		Queue:    q,
		Balancer: b,
	}, nil
}

// NewMemoryClient cria um cliente usando apenas memória
func NewMemoryClient(appName string) (*Client, error) {
	config := config.NewMemoryConfig(appName)
	return NewClientWithConfig(config)
}

// NewRedisClient cria um cliente usando Redis
func NewRedisClient(appName, redisAddr string) (*Client, error) {
	config := config.NewRedisConfig(appName, redisAddr)
	return NewClientWithConfig(config)
}

// NewHybridClient cria um cliente híbrido (queue Redis, balancer Memory)
func NewHybridClient(appName, redisAddr string) (*Client, error) {
	config := config.NewHybridConfig(appName, redisAddr)
	return NewClientWithConfig(config)
}

func (b *Client) Publish(event string, payload any) error {
	return b.Queue.Store(event, payload)
}

func (b *Client) EventBrokerKey(eventName string) string {
	const brokerName = "gqueue"
	return fmt.Sprintf("%s:%s:%s", brokerName, b.appName, eventName)
}

// GetConfig retorna a configuração atual
func (b *Client) GetConfig() *config.Config {
	return b.config
}

// Health verifica a saúde dos componentes
func (b *Client) Health() error {
	return factory.ComponentsHealth(b.Queue, b.Balancer)
}

// Close fecha todas as conexões
func (b *Client) Close() error {
	return factory.CloseComponents(b.Queue, b.Balancer)
}

// GetStats retorna estatísticas dos componentes
func (b *Client) GetStats() (map[string]interface{}, error) {
	stats := make(map[string]interface{})

	// Stats da queue
	queueLen, err := b.Queue.Len()
	if err != nil {
		return nil, fmt.Errorf("failed to get queue length: %w", err)
	}
	stats["queue_length"] = queueLen

	// Stats do balancer
	totalSubs, err := b.Balancer.GetTotalSubscribers()
	if err != nil {
		return nil, fmt.Errorf("failed to get total subscribers: %w", err)
	}
	stats["total_subscribers"] = totalSubs

	activeSubs, err := b.Balancer.GetActiveSubscribers()
	if err != nil {
		return nil, fmt.Errorf("failed to get active subscribers: %w", err)
	}
	stats["active_subscribers"] = len(activeSubs)

	return stats, nil
}

// PrintConfig imprime a configuração atual
func (b *Client) PrintConfig() {
	b.config.Print()
}
