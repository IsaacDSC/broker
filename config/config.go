package config

import (
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/IsaacDSC/broker/load"
	"github.com/IsaacDSC/broker/queue"
)

// Config configuração geral da aplicação
type Config struct {
	AppName  string
	Queue    *QueueConfig
	Balancer *BalancerConfig
}

// QueueConfig configuração da fila
type QueueConfig struct {
	Type   queue.QueueType
	Redis  *RedisConfig
	Prefix string
}

// BalancerConfig configuração do balancer
type BalancerConfig struct {
	Type          load.BalancerType
	Redis         *RedisConfig
	Strategy      load.BalanceStrategy
	HeartbeatTTL  time.Duration
	CleanupTicker time.Duration
	Prefix        string
}

// RedisConfig configuração do Redis
type RedisConfig struct {
	Addr         string
	Password     string
	DB           int
	PoolSize     int
	MaxRetries   int
	MinIdleConns int
}

// LoadConfig carrega configurações do ambiente ou usa padrões
func LoadConfig() *Config {
	appName := getEnvString("APP_NAME", "broker-app")

	return &Config{
		AppName:  appName,
		Queue:    loadQueueConfig(appName),
		Balancer: loadBalancerConfig(appName),
	}
}

func loadQueueConfig(appName string) *QueueConfig {
	queueType := queue.QueueType(getEnvString("QUEUE_TYPE", string(queue.QueueTypeMemory)))
	prefix := getEnvString("QUEUE_PREFIX", "queue")

	config := &QueueConfig{
		Type:   queueType,
		Prefix: prefix,
	}

	// Se for Redis, carrega configurações específicas
	if queueType == queue.QueueTypeRedis {
		config.Redis = loadRedisConfig("QUEUE_REDIS")
	}

	return config
}

func loadBalancerConfig(appName string) *BalancerConfig {
	balancerType := load.BalancerType(getEnvString("BALANCER_TYPE", string(load.BalancerTypeMemory)))
	strategy := load.BalanceStrategy(getEnvString("BALANCE_STRATEGY", string(load.StrategyRoundRobin)))
	prefix := getEnvString("BALANCER_PREFIX", "balancer")

	config := &BalancerConfig{
		Type:          balancerType,
		Strategy:      strategy,
		HeartbeatTTL:  getEnvDuration("HEARTBEAT_TTL", 30*time.Second),
		CleanupTicker: getEnvDuration("CLEANUP_TICKER", 60*time.Second),
		Prefix:        prefix,
	}

	// Se for Redis, carrega configurações específicas
	if balancerType == load.BalancerTypeRedis {
		config.Redis = loadRedisConfig("BALANCER_REDIS")
	}

	return config
}

func loadRedisConfig(prefix string) *RedisConfig {
	return &RedisConfig{
		Addr:         getEnvString(prefix+"_ADDR", "localhost:6379"),
		Password:     getEnvString(prefix+"_PASSWORD", ""),
		DB:           getEnvInt(prefix+"_DB", 0),
		PoolSize:     getEnvInt(prefix+"_POOL_SIZE", 10),
		MaxRetries:   getEnvInt(prefix+"_MAX_RETRIES", 3),
		MinIdleConns: getEnvInt(prefix+"_MIN_IDLE_CONNS", 2),
	}
}

// ToQueueConfig converte para queue.QueueConfig
func (qc *QueueConfig) ToQueueConfig(appName string) *queue.QueueConfig {
	config := &queue.QueueConfig{
		Type:    qc.Type,
		AppName: appName,
		Prefix:  qc.Prefix,
	}

	if qc.Redis != nil {
		config.Redis = &queue.RedisConfig{
			Addr:         qc.Redis.Addr,
			Password:     qc.Redis.Password,
			DB:           qc.Redis.DB,
			PoolSize:     qc.Redis.PoolSize,
			MaxRetries:   qc.Redis.MaxRetries,
			MinIdleConns: qc.Redis.MinIdleConns,
		}
	}

	return config
}

// ToBalancerConfig converte para load.BalancerConfig
func (bc *BalancerConfig) ToBalancerConfig(appName string) *load.BalancerConfig {
	config := &load.BalancerConfig{
		Type:          bc.Type,
		AppName:       appName,
		Strategy:      bc.Strategy,
		HeartbeatTTL:  bc.HeartbeatTTL,
		CleanupTicker: bc.CleanupTicker,
		Prefix:        bc.Prefix,
	}

	if bc.Redis != nil {
		config.Redis = &load.RedisConfig{
			Addr:         bc.Redis.Addr,
			Password:     bc.Redis.Password,
			DB:           bc.Redis.DB,
			PoolSize:     bc.Redis.PoolSize,
			MaxRetries:   bc.Redis.MaxRetries,
			MinIdleConns: bc.Redis.MinIdleConns,
		}
	}

	return config
}

// Validate valida a configuração
func (c *Config) Validate() error {
	if c.AppName == "" {
		return fmt.Errorf("app name is required")
	}

	// Valida configuração da queue
	if c.Queue.Type != queue.QueueTypeMemory && c.Queue.Type != queue.QueueTypeRedis {
		return fmt.Errorf("invalid queue type: %s", c.Queue.Type)
	}

	if c.Queue.Type == queue.QueueTypeRedis && c.Queue.Redis == nil {
		return fmt.Errorf("redis config is required for redis queue")
	}

	// Valida configuração do balancer
	if c.Balancer.Type != load.BalancerTypeMemory && c.Balancer.Type != load.BalancerTypeRedis {
		return fmt.Errorf("invalid balancer type: %s", c.Balancer.Type)
	}

	if c.Balancer.Type == load.BalancerTypeRedis && c.Balancer.Redis == nil {
		return fmt.Errorf("redis config is required for redis balancer")
	}

	// Valida estratégias
	validStrategies := map[load.BalanceStrategy]bool{
		load.StrategyRoundRobin:     true,
		load.StrategyConsistentHash: true,
		load.StrategyWeighted:       true,
		load.StrategyLeastConn:      true,
	}

	if !validStrategies[c.Balancer.Strategy] {
		return fmt.Errorf("invalid balance strategy: %s", c.Balancer.Strategy)
	}

	return nil
}

// Print imprime a configuração atual
func (c *Config) Print() {
	fmt.Printf("=== Broker Configuration ===\n")
	fmt.Printf("App Name: %s\n", c.AppName)
	fmt.Printf("Queue Type: %s\n", c.Queue.Type)
	fmt.Printf("Balancer Type: %s\n", c.Balancer.Type)
	fmt.Printf("Balance Strategy: %s\n", c.Balancer.Strategy)
	fmt.Printf("Heartbeat TTL: %v\n", c.Balancer.HeartbeatTTL)
	fmt.Printf("Cleanup Ticker: %v\n", c.Balancer.CleanupTicker)

	if c.Queue.Type == queue.QueueTypeRedis && c.Queue.Redis != nil {
		fmt.Printf("Queue Redis: %s (DB: %d)\n", c.Queue.Redis.Addr, c.Queue.Redis.DB)
	}

	if c.Balancer.Type == load.BalancerTypeRedis && c.Balancer.Redis != nil {
		fmt.Printf("Balancer Redis: %s (DB: %d)\n", c.Balancer.Redis.Addr, c.Balancer.Redis.DB)
	}
	fmt.Printf("===========================\n")
}

// Funções auxiliares para ler variáveis de ambiente

func getEnvString(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return defaultValue
}

func getEnvDuration(key string, defaultValue time.Duration) time.Duration {
	if value := os.Getenv(key); value != "" {
		if duration, err := time.ParseDuration(value); err == nil {
			return duration
		}
	}
	return defaultValue
}

// Configurações pré-definidas para diferentes ambientes

// NewMemoryConfig cria configuração para usar apenas memória
func NewMemoryConfig(appName string) *Config {
	return &Config{
		AppName: appName,
		Queue: &QueueConfig{
			Type:   queue.QueueTypeMemory,
			Prefix: "queue",
		},
		Balancer: &BalancerConfig{
			Type:          load.BalancerTypeMemory,
			Strategy:      load.StrategyRoundRobin,
			HeartbeatTTL:  30 * time.Second,
			CleanupTicker: 60 * time.Second,
			Prefix:        "balancer",
		},
	}
}

// NewRedisConfig cria configuração para usar Redis
func NewRedisConfig(appName, redisAddr string) *Config {
	redisConfig := &RedisConfig{
		Addr:         redisAddr,
		Password:     "",
		DB:           0,
		PoolSize:     10,
		MaxRetries:   3,
		MinIdleConns: 2,
	}

	return &Config{
		AppName: appName,
		Queue: &QueueConfig{
			Type:   queue.QueueTypeRedis,
			Redis:  redisConfig,
			Prefix: "queue",
		},
		Balancer: &BalancerConfig{
			Type:          load.BalancerTypeRedis,
			Redis:         redisConfig,
			Strategy:      load.StrategyConsistentHash,
			HeartbeatTTL:  30 * time.Second,
			CleanupTicker: 60 * time.Second,
			Prefix:        "balancer",
		},
	}
}

// NewHybridConfig cria configuração híbrida (queue Redis, balancer Memory)
func NewHybridConfig(appName, redisAddr string) *Config {
	return &Config{
		AppName: appName,
		Queue: &QueueConfig{
			Type: queue.QueueTypeRedis,
			Redis: &RedisConfig{
				Addr:         redisAddr,
				Password:     "",
				DB:           0,
				PoolSize:     10,
				MaxRetries:   3,
				MinIdleConns: 2,
			},
			Prefix: "queue",
		},
		Balancer: &BalancerConfig{
			Type:          load.BalancerTypeMemory,
			Strategy:      load.StrategyRoundRobin,
			HeartbeatTTL:  30 * time.Second,
			CleanupTicker: 60 * time.Second,
			Prefix:        "balancer",
		},
	}
}
