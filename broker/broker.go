package broker

import (
	"context"
	"log"
	"sync"

	"github.com/IsaacDSC/broker/queue"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

type Client struct {
	ID      uuid.UUID
	appName string
	Queue   *queue.Redis

	// Event >> Handlers Consumer
	subScribers map[string]SubscriberHandler
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
		ID:          uuid.New(),
		appName:     cfg.AppName,
		Queue:       queue.NewRedis(rdb),
		subScribers: make(map[string]SubscriberHandler),
	}
	return client
}

func (b *Client) Publish(ctx context.Context, event string, payload any) error {
	return b.Queue.SaveEventMsg(ctx, event, payload, queue.ActiveStatus)
}

// Consumer
type Ctx struct {
	ctx     context.Context
	payload any
}

func (ctx Ctx) GetPayload() any {
	return ctx.payload
}

type SubscriberHandler func(subctx Ctx) error

type Subscriber struct {
	Event   string
	Handler SubscriberHandler
}

func (b *Client) Subscribe(event string, handler SubscriberHandler) *Client {
	b.subScribers[event] = handler
	return b
}

func (b *Client) Listener() error {
	ctx := context.Background()
	for event := range b.subScribers {
		if err := b.Queue.SetQueue(ctx, event); err != nil {
			return err
		}
	}

	const limitMsgs = 5

	var wg sync.WaitGroup

	for {
		for eventName := range b.subScribers {
			wg.Add(1)

			go func() {
				defer wg.Done()

				msgs, err := b.Queue.GetMsgsToProcess(ctx, eventName, limitMsgs)
				if err != nil {
					log.Println("[*][ERROR] error on get msgs by queue with event:", eventName, err)
				}

				for _, msg := range msgs {
					handler, ok := b.subScribers[eventName]

					if !ok {
						log.Println("[*][WARN] handler not found for event:", eventName)
						return
					}

					if err := handler(Ctx{ctx: ctx, payload: msg.Value}); err != nil {
						log.Println("[*][ERROR] error on handler with event:", eventName, err)
						if err := b.Queue.ChangeStatus(ctx, eventName, queue.ProcessingStatus, queue.NackedStatus, msg.Time.UnixMicro(), msg.Time.UnixMicro()); err != nil {
							log.Println("[*][ERROR] error on update status with event:", eventName, err)
						}
						return
					} else {
						if err := b.Queue.ChangeStatus(ctx, eventName, queue.ProcessingStatus, queue.AckedStatus, msg.Time.UnixMicro(), msg.Time.UnixMicro()); err != nil {
							log.Println("[*][ERROR] error on update status with event:", eventName, err)
						}
					}

				}
			}()
		}

		wg.Wait()
	}

}

func (b *Client) Close() error {
	return b.Queue.Close()
}
