package sub

import (
	"context"
	"errors"
	"log"
	"sync"

	"github.com/IsaacDSC/broker/broker"
	"github.com/google/uuid"
)

const (
	MaxConcurrency = 10
)

type Subscribe struct {
	*broker.Client
	subID        uuid.UUID
	subscribers  map[string]Subscriber
	semaphore    chan struct{}
	stopCh       chan struct{}
	processingWg sync.WaitGroup
}

func NewSubscribe(conn *broker.Client) *Subscribe {
	sub := &Subscribe{
		Client:      conn,
		subID:       uuid.New(),
		subscribers: make(map[string]Subscriber),
		semaphore:   make(chan struct{}, MaxConcurrency),
		stopCh:      make(chan struct{}),
	}

	return sub
}

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

func (b *Subscribe) WithSubscriber(eventName string, handler SubscriberHandler) *Subscribe {
	key := b.EventBrokerKey(eventName)
	b.subscribers[key] = Subscriber{
		Event:   eventName,
		Handler: handler,
	}
	return b
}

func (b *Subscribe) Listener() error {
	if b.subscribers == nil {
		return errors.New("subscribers map is nil")
	}

	if len(b.subscribers) == 0 {
		return errors.New("no subscribers")
	}

	for {
		//criar fila por event_name
		// b.Queue.
	}
}

func (b *Subscribe) processMessage(messageID, eventName string, msg any) {
	defer func() {
		<-b.semaphore // libera o slot do semáforo
		b.processingWg.Done()

		// Marca a mensagem como processada após o processamento
		b.Queue.MarkAsProcessed(messageID)
	}()

	subscriber, ok := b.subscribers[b.EventBrokerKey(eventName)]
	if !ok {
		log.Println("[*] Not subscriber on event: ", eventName)
		return
	}

	ctx := context.Background()
	if err := subscriber.Handler(Ctx{ctx, msg}); err != nil {
		log.Println("[*] Error handling event: ", eventName, err)
	}
}
