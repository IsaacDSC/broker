package broker

import (
	"fmt"

	"github.com/IsaacDSC/broker/load"
	"github.com/IsaacDSC/broker/queue"
	"github.com/google/uuid"
)

type Client struct {
	ID      uuid.UUID
	appName string

	Queue    queue.Queuer
	Balancer *load.Balancer
}

type Config struct {
	Addr    string
	AppName string
}

func NewClient(cfg Config) *Client {
	client := &Client{
		ID:       uuid.New(),
		appName:  cfg.AppName,
		Queue:    queue.NewQueueBase(),
		Balancer: load.NewBalancer(cfg.AppName),
	}
	return client
}

func NewRdbClient(cfg RedisConfig) *Client {
	client := &Client{
		ID:       uuid.New(),
		appName:  cfg.AppName,
		Queue:    queue.NewQueueBase(),
		Balancer: load.NewBalancer(cfg.AppName),
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
