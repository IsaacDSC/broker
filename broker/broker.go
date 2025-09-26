package broker

import (
	"fmt"

	"github.com/IsaacDSC/broker/queue"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

type Client struct {
	ID      uuid.UUID
	appName string

	Queue queue.Queuer
}

type Config struct {
	Addr    string
	AppName string
}

func NewClient(cfg RedisConfig) *Client {
	rdb := redis.NewClient(&redis.Options{
		Addr:     cfg.Addr,
		Password: cfg.Password,
		DB:       cfg.DB,
	})

	client := &Client{
		ID:      uuid.New(),
		appName: cfg.AppName,
		Queue:   queue.NewRedis(rdb),
	}
	return client
}

func (b *Client) Publish(event string, payload any) error {
	b.Queue.Store(event, payload)
	return nil
}

func (b *Client) EventBrokerKey(eventName string) string {
	const brokerName = "gqueue"
	return fmt.Sprintf("%s:%s:%s", brokerName, b.appName, eventName)
}
