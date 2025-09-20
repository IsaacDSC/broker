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

	// Registra este subscriber no balancer
	if err := conn.Balancer.Subscribe(sub.subID); err != nil {
		log.Printf("Failed to register subscriber: %v", err)
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

func (b *Subscribe) processMessage(messageID, eventName string, msg any) {
	defer func() {
		<-b.semaphore // libera o slot do semáforo
		b.processingWg.Done()

		// Marca a mensagem como processada após o processamento
		if err := b.Queue.MarkAsProcessed(messageID); err != nil {
			log.Printf("Failed to mark message as processed: %v", err)
		}

		// Atualiza heartbeat do subscriber
		if err := b.Balancer.UpdateHeartbeat(b.subID); err != nil {
			log.Printf("Failed to update heartbeat: %v", err)
		}
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

func (b *Subscribe) Listener() error {
	if b.subscribers == nil {
		return errors.New("subscribers map is nil")
	}

	if len(b.subscribers) == 0 {
		return errors.New("no subscribers")
	}

	for {
		select {
		case <-b.stopCh:
			// Desregistra do balancer antes de parar
			if err := b.Balancer.Unsubscribe(b.subID); err != nil {
				log.Printf("Failed to unsubscribe: %v", err)
			}
			// Aguarda todas as goroutines terminarem antes de parar
			b.processingWg.Wait()
			return nil
		default:
			// Processa mensagens da fila usando o novo sistema
			b.processAvailableMessages()
		}
	}
}

func (b *Subscribe) processAvailableMessages() {
	// Para cada tipo de evento que este subscriber está inscrito
	for eventKey := range b.subscribers {
		// Extrai o nome do evento da chave
		eventName := b.extractEventName(eventKey)

		// Busca mensagens não reivindicadas para este evento
		messages, err := b.Queue.GetUnclaimedMessagesByKey(eventName)
		if err != nil {
			log.Printf("Failed to get messages for event %s: %v", eventName, err)
			continue
		}

		for _, msg := range messages {
			// Verifica se este subscriber deve processar esta mensagem
			shouldProcess, err := b.Balancer.ClaimMessage(b.subID, msg.ID)
			if err != nil || !shouldProcess {
				continue
			}

			// Verifica se a mensagem já foi processada
			if b.Queue.IsProcessed(msg.ID) {
				continue
			}

			// Tenta reivindicar a mensagem atomicamente
			if !b.Queue.ClaimMessage(msg.ID) {
				continue
			}

			select {
			case b.semaphore <- struct{}{}: // tenta adquirir slot do semáforo
				b.processingWg.Add(1)
				go b.processMessage(msg.ID, eventName, msg.Value)
			default:
				// Se não conseguir adquirir o semáforo, libera a reivindicação
				// e tenta novamente no próximo ciclo
				continue
			}
		}
	}
}

// Extrai o nome do evento da chave do broker
func (b *Subscribe) extractEventName(eventKey string) string {
	// Remove o prefixo do broker (formato: "gqueue:app-name:event")
	parts := []rune(eventKey)
	lastColon := -1

	for i := len(parts) - 1; i >= 0; i-- {
		if parts[i] == ':' {
			lastColon = i
			break
		}
	}

	if lastColon != -1 && lastColon < len(parts)-1 {
		return string(parts[lastColon+1:])
	}

	return eventKey
}

func (b *Subscribe) Stop() {
	close(b.stopCh)
}

// GetSubscriberID retorna o ID único deste subscriber
func (b *Subscribe) GetSubscriberID() uuid.UUID {
	return b.subID
}

// GetSubscriberInfo retorna informações sobre este subscriber
func (b *Subscribe) GetSubscriberInfo() (interface{}, error) {
	return b.Balancer.GetSubscriberInfo(b.subID)
}

// SetWeight define o peso deste subscriber para balanceamento
func (b *Subscribe) SetWeight(weight int) error {
	return b.Balancer.SetSubscriberWeight(b.subID, weight)
}

// UpdateHeartbeat força uma atualização do heartbeat
func (b *Subscribe) UpdateHeartbeat() error {
	return b.Balancer.UpdateHeartbeat(b.subID)
}
