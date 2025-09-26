package broker

import (
	"context"
	"time"

	"github.com/IsaacDSC/broker/queue"
	"github.com/redis/go-redis/v9"
)

// RedisConfig holds configuration for Redis queue
type RedisConfig struct {
	AppName string
	// Redis connection settings
	Addr     string `json:"addr"`     // Redis server address (default: "localhost:6379")
	Password string `json:"password"` // Redis password (default: "")
	DB       int    `json:"db"`       // Redis database number (default: 0)

	// Connection pool settings
	PoolSize     int           `json:"pool_size"`      // Maximum number of connections (default: 10)
	MinIdleConns int           `json:"min_idle_conns"` // Minimum number of idle connections (default: 0)
	MaxRetries   int           `json:"max_retries"`    // Maximum number of retries (default: 3)
	DialTimeout  time.Duration `json:"dial_timeout"`   // Timeout for establishing connection (default: 5s)
	ReadTimeout  time.Duration `json:"read_timeout"`   // Timeout for socket reads (default: 3s)
	WriteTimeout time.Duration `json:"write_timeout"`  // Timeout for socket writes (default: 3s)

	// Queue-specific settings
	KeyPrefix string `json:"key_prefix"` // Prefix for all Redis keys (default: "queue:")
}

// DefaultRedisConfig returns a Redis configuration with sensible defaults
func DefaultRedisConfig(appName string) *RedisConfig {
	return &RedisConfig{
		AppName:      appName,
		Addr:         "localhost:6379",
		Password:     "",
		DB:           0,
		PoolSize:     10,
		MinIdleConns: 0,
		MaxRetries:   3,
		DialTimeout:  5 * time.Second,
		ReadTimeout:  3 * time.Second,
		WriteTimeout: 3 * time.Second,
		KeyPrefix:    "queue:",
	}
}

// NewRedisClient creates a new Redis client from the configuration
func (c *RedisConfig) NewRedisClient() *redis.Client {
	return redis.NewClient(&redis.Options{
		Addr:         c.Addr,
		Password:     c.Password,
		DB:           c.DB,
		PoolSize:     c.PoolSize,
		MinIdleConns: c.MinIdleConns,
		MaxRetries:   c.MaxRetries,
		DialTimeout:  c.DialTimeout,
		ReadTimeout:  c.ReadTimeout,
		WriteTimeout: c.WriteTimeout,
	})
}

// NewRedisQueue creates a new Redis queue with the given configuration
func (c *RedisConfig) NewRedisQueue() (*queue.Redis, error) {
	client := c.NewRedisClient()

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), c.DialTimeout)
	defer cancel()

	_, err := client.Ping(ctx).Result()
	if err != nil {
		client.Close()
		return nil, err
	}

	return queue.NewRedis(client), nil
}

// NewRedisQueueWithClient creates a new Redis queue with an existing client
func NewRedisQueueWithClient(client *redis.Client) *queue.Redis {
	return queue.NewRedis(client)
}

// TestRedisConnection tests if Redis is available with the given configuration
func (c *RedisConfig) TestRedisConnection() error {
	client := c.NewRedisClient()
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), c.DialTimeout)
	defer cancel()

	_, err := client.Ping(ctx).Result()
	return err
}

// RedisClusterConfig holds configuration for Redis Cluster queue
type RedisClusterConfig struct {
	// Cluster node addresses
	Addrs    []string `json:"addrs"`    // List of cluster node addresses
	Password string   `json:"password"` // Redis password (default: "")

	// Connection pool settings
	PoolSize     int           `json:"pool_size"`      // Maximum number of connections per node (default: 5)
	MinIdleConns int           `json:"min_idle_conns"` // Minimum number of idle connections per node (default: 0)
	MaxRetries   int           `json:"max_retries"`    // Maximum number of retries (default: 3)
	DialTimeout  time.Duration `json:"dial_timeout"`   // Timeout for establishing connection (default: 5s)
	ReadTimeout  time.Duration `json:"read_timeout"`   // Timeout for socket reads (default: 3s)
	WriteTimeout time.Duration `json:"write_timeout"`  // Timeout for socket writes (default: 3s)

	// Queue-specific settings
	KeyPrefix string `json:"key_prefix"` // Prefix for all Redis keys (default: "queue:")
}

// DefaultRedisClusterConfig returns a Redis Cluster configuration with sensible defaults
func DefaultRedisClusterConfig(addrs []string) *RedisClusterConfig {
	return &RedisClusterConfig{
		Addrs:        addrs,
		Password:     "",
		PoolSize:     5,
		MinIdleConns: 0,
		MaxRetries:   3,
		DialTimeout:  5 * time.Second,
		ReadTimeout:  3 * time.Second,
		WriteTimeout: 3 * time.Second,
		KeyPrefix:    "queue:",
	}
}

// NewRedisClusterClient creates a new Redis Cluster client from the configuration
func (c *RedisClusterConfig) NewRedisClusterClient() *redis.ClusterClient {
	return redis.NewClusterClient(&redis.ClusterOptions{
		Addrs:        c.Addrs,
		Password:     c.Password,
		PoolSize:     c.PoolSize,
		MinIdleConns: c.MinIdleConns,
		MaxRetries:   c.MaxRetries,
		DialTimeout:  c.DialTimeout,
		ReadTimeout:  c.ReadTimeout,
		WriteTimeout: c.WriteTimeout,
	})
}

// NewRedisClusterQueue creates a new Redis queue with Redis Cluster
func (c *RedisClusterConfig) NewRedisClusterQueue() (*queue.Redis, error) {
	client := c.NewRedisClusterClient()

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), c.DialTimeout)
	defer cancel()

	_, err := client.Ping(ctx).Result()
	if err != nil {
		client.Close()
		return nil, err
	}

	// Redis Cluster client can be used as a regular redis.Cmdable interface
	return queue.NewRedis(client), nil
}

// TestRedisClusterConnection tests if Redis Cluster is available with the given configuration
func (c *RedisClusterConfig) TestRedisClusterConnection() error {
	client := c.NewRedisClusterClient()
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), c.DialTimeout)
	defer cancel()

	_, err := client.Ping(ctx).Result()
	return err
}

// RedisConnectionInfo holds information about Redis connection
type RedisConnectionInfo struct {
	Connected bool   `json:"connected"`
	Address   string `json:"address"`
	Database  int    `json:"database"`
	Error     string `json:"error,omitempty"`
}

// GetRedisConnectionInfo returns information about the Redis connection
func (c *RedisConfig) GetRedisConnectionInfo() RedisConnectionInfo {
	info := RedisConnectionInfo{
		Address:  c.Addr,
		Database: c.DB,
	}

	err := c.TestRedisConnection()
	if err != nil {
		info.Connected = false
		info.Error = err.Error()
	} else {
		info.Connected = true
	}

	return info
}

// GetRedisClusterConnectionInfo returns information about the Redis Cluster connection
func (c *RedisClusterConfig) GetRedisClusterConnectionInfo() RedisConnectionInfo {
	info := RedisConnectionInfo{
		Address: "cluster:" + string(rune(len(c.Addrs))) + "_nodes",
	}

	err := c.TestRedisClusterConnection()
	if err != nil {
		info.Connected = false
		info.Error = err.Error()
	} else {
		info.Connected = true
	}

	return info
}
