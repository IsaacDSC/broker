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

type Broker struct {
	*broker.Client
	subID        uuid.UUID
	subscribers  map[string]Subscriber
	semaphore    chan struct{}
	stopCh       chan struct{}
	processingWg sync.WaitGroup
}

func NewBroker(conn *broker.Client) *Broker {
	return &Broker{
		Client:      conn,
		subID:       uuid.New(),
		subscribers: make(map[string]Subscriber),
		semaphore:   make(chan struct{}, MaxConcurrency),
		stopCh:      make(chan struct{}),
	}
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

func (b *Broker) WithSubscriber(eventName string, handler SubscriberHandler) *Broker {
	key := b.EventBrokerKey(eventName)
	b.subscribers[key] = Subscriber{
		Event:   eventName,
		Handler: handler,
	}
	return b
}

func (b *Broker) processMessage(eventName string, msg any) {
	defer func() {
		<-b.semaphore // libera o slot do semáforo
		b.processingWg.Done()
	}()

	subscriber, ok := b.subscribers[b.EventBrokerKey(eventName)]
	if !ok {
		log.Println("[*] Not subscriber on event: ", eventName)
		b.Queue.Delete(eventName)
		return
	}

	ctx := context.Background()
	if err := subscriber.Handler(Ctx{ctx, msg}); err != nil {
		log.Println("[*] Error handling event: ", eventName, err)
	}
}

func (b *Broker) Listener() error {
	if b.subscribers == nil {
		return errors.New("subscribers map is nil")
	}

	if len(b.subscribers) == 0 {
		return errors.New("no subscribers")
	}

	for {
		select {
		case <-b.stopCh:
			// Aguarda todas as goroutines terminarem antes de parar
			b.processingWg.Wait()
			return nil
		default:
			// Processa mensagens da fila
			b.Queue.Range(func(eventName string, msg any) bool {
				select {
				case b.semaphore <- struct{}{}: // tenta adquirir slot do semáforo
					// Remove a mensagem da fila imediatamente após adquirir o semáforo
					b.Queue.Delete(eventName)

					b.processingWg.Add(1)
					go b.processMessage(eventName, msg)

					return true // continua iterando
				default:
					// Se não conseguir adquirir o semáforo, para a iteração
					// e tenta novamente no próximo ciclo do loop principal
					return false
				}
			})
		}
	}
}

func (b *Broker) Stop() {
	close(b.stopCh)
}
