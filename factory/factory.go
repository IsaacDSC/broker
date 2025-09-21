package factory

import (
	"fmt"

	"github.com/IsaacDSC/broker/config"
	"github.com/IsaacDSC/broker/load"
	"github.com/IsaacDSC/broker/queue"
)

// Factory interface principal para criação de componentes
type Factory interface {
	CreateQueue(config *config.Config) (queue.Queue, error)
	CreateBalancer(config *config.Config) (load.Balancer, error)
}

// DefaultFactory implementação padrão da factory
type DefaultFactory struct{}

// NewFactory cria uma nova instância da factory
func NewFactory() Factory {
	return &DefaultFactory{}
}

// CreateQueue cria uma instância de Queue baseada na configuração
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

// CreateBalancer cria uma instância de Balancer baseada na configuração
func (f *DefaultFactory) CreateBalancer(config *config.Config) (load.Balancer, error) {
	balancerConfig := config.Balancer.ToBalancerConfig(config.AppName)

	switch balancerConfig.Type {
	case load.BalancerTypeMemory:
		return load.NewMemoryBalancer(balancerConfig), nil
	case load.BalancerTypeRedis:
		return load.NewRedisBalancer(balancerConfig)
	default:
		return nil, fmt.Errorf("unsupported balancer type: %s", balancerConfig.Type)
	}
}

// QueueFactory factory específica para Queue
type QueueFactory struct{}

// NewQueueFactory cria uma nova factory de Queue
func NewQueueFactory() *QueueFactory {
	return &QueueFactory{}
}

// Create cria uma Queue baseada na configuração
func (qf *QueueFactory) Create(config *config.Config) (queue.Queue, error) {
	return NewFactory().CreateQueue(config)
}

// CreateMemoryQueue cria uma Queue em memória
func (qf *QueueFactory) CreateMemoryQueue(appName string) queue.Queue {
	config := &queue.QueueConfig{
		Type:    queue.QueueTypeMemory,
		AppName: appName,
		Prefix:  "queue",
	}
	return queue.NewMemoryQueue(config)
}

// CreateRedisQueue cria uma Queue Redis
func (qf *QueueFactory) CreateRedisQueue(appName, redisAddr string) (queue.Queue, error) {
	config := &queue.QueueConfig{
		Type:    queue.QueueTypeRedis,
		AppName: appName,
		Prefix:  "queue",
		Redis: &queue.RedisConfig{
			Addr:         redisAddr,
			Password:     "",
			DB:           0,
			PoolSize:     10,
			MaxRetries:   3,
			MinIdleConns: 2,
		},
	}
	return queue.NewRedisQueue(config)
}

// BalancerFactory factory específica para Balancer
type BalancerFactory struct{}

// NewBalancerFactory cria uma nova factory de Balancer
func NewBalancerFactory() *BalancerFactory {
	return &BalancerFactory{}
}

// Create cria um Balancer baseado na configuração
func (bf *BalancerFactory) Create(config *config.Config) (load.Balancer, error) {
	return NewFactory().CreateBalancer(config)
}

// CreateMemoryBalancer cria um Balancer em memória
func (bf *BalancerFactory) CreateMemoryBalancer(appName string, strategy load.BalanceStrategy) load.Balancer {
	config := &load.BalancerConfig{
		Type:     load.BalancerTypeMemory,
		AppName:  appName,
		Strategy: strategy,
		Prefix:   "balancer",
	}
	return load.NewMemoryBalancer(config)
}

// CreateRedisBalancer cria um Balancer Redis
func (bf *BalancerFactory) CreateRedisBalancer(appName, redisAddr string, strategy load.BalanceStrategy) (load.Balancer, error) {
	config := &load.BalancerConfig{
		Type:     load.BalancerTypeRedis,
		AppName:  appName,
		Strategy: strategy,
		Prefix:   "balancer",
		Redis: &load.RedisConfig{
			Addr:         redisAddr,
			Password:     "",
			DB:           0,
			PoolSize:     10,
			MaxRetries:   3,
			MinIdleConns: 2,
		},
	}
	return load.NewRedisBalancer(config)
}

// BrokerComponentsFactory factory para criar todos os componentes do broker
type BrokerComponentsFactory struct {
	config *config.Config
}

// NewBrokerComponentsFactory cria uma nova factory de componentes do broker
func NewBrokerComponentsFactory(config *config.Config) *BrokerComponentsFactory {
	return &BrokerComponentsFactory{
		config: config,
	}
}

// CreateComponents cria todos os componentes necessários
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
		// Fecha a queue se o balancer falhar
		q.Close()
		return nil, nil, fmt.Errorf("failed to create balancer: %w", err)
	}

	return q, b, nil
}

// ValidateAndCreate valida a configuração e cria os componentes
func (bcf *BrokerComponentsFactory) ValidateAndCreate() (queue.Queue, load.Balancer, error) {
	if err := bcf.config.Validate(); err != nil {
		return nil, nil, fmt.Errorf("invalid configuration: %w", err)
	}

	return bcf.CreateComponents()
}

// ComponentsHealth verifica a saúde dos componentes
func ComponentsHealth(q queue.Queue, b load.Balancer) error {
	if err := q.Health(); err != nil {
		return fmt.Errorf("queue health check failed: %w", err)
	}

	if err := b.Health(); err != nil {
		return fmt.Errorf("balancer health check failed: %w", err)
	}

	return nil
}

// CloseComponents fecha todos os componentes com segurança
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
