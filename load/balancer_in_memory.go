package load

import (
	"sync"
	"sync/atomic"

	"github.com/google/uuid"
)

type Balancer struct {
	appName     string
	subscribers map[uuid.UUID]*Subscriber
	mutex       sync.RWMutex
	counter     int64
}

type Subscriber struct {
	ID     uuid.UUID
	Active bool
}

func NewBalancer(appName string) *Balancer {
	return &Balancer{
		appName:     appName,
		subscribers: make(map[uuid.UUID]*Subscriber),
		counter:     0,
	}
}

func (b *Balancer) Subscribe(subID uuid.UUID) error {
	b.mutex.Lock()
	defer b.mutex.Unlock()

	b.subscribers[subID] = &Subscriber{
		ID:     subID,
		Active: true,
	}
	return nil
}

func (b *Balancer) Unsubscribe(subID uuid.UUID) error {
	b.mutex.Lock()
	defer b.mutex.Unlock()

	if sub, exists := b.subscribers[subID]; exists {
		sub.Active = false
		delete(b.subscribers, subID)
	}
	return nil
}

func (b *Balancer) GetTotalSubscribers() int {
	b.mutex.RLock()
	defer b.mutex.RUnlock()

	count := 0
	for _, sub := range b.subscribers {
		if sub.Active {
			count++
		}
	}
	return count
}

// ShouldProcess determina se este subscriber deve processar a mensagem
// usando round-robin baseado no hash da mensagem
func (b *Balancer) ShouldProcess(subID uuid.UUID, messageKey string) bool {
	b.mutex.RLock()
	defer b.mutex.RUnlock()

	activeSubscribers := make([]uuid.UUID, 0)
	for id, sub := range b.subscribers {
		if sub.Active {
			activeSubscribers = append(activeSubscribers, id)
		}
	}

	if len(activeSubscribers) == 0 {
		return false
	}

	// Usa round-robin baseado no counter atômico
	index := atomic.AddInt64(&b.counter, 1) % int64(len(activeSubscribers))
	selectedID := activeSubscribers[index]

	return selectedID == subID
}

// ClaimMessage tenta reivindicar uma mensagem para processamento
// Retorna true se este subscriber deve processar a mensagem
func (b *Balancer) ClaimMessage(subID uuid.UUID, messageID string) bool {
	b.mutex.RLock()
	defer b.mutex.RUnlock()

	// Verifica se o subscriber está ativo
	sub, exists := b.subscribers[subID]
	if !exists || !sub.Active {
		return false
	}

	activeCount := 0
	subscriberIndex := -1
	currentIndex := 0

	for id, subscriber := range b.subscribers {
		if subscriber.Active {
			if id == subID {
				subscriberIndex = currentIndex
			}
			activeCount++
			currentIndex++
		}
	}

	if activeCount == 0 || subscriberIndex == -1 {
		return false
	}

	// Usa hash simples do messageID para distribuição consistente
	hash := b.simpleHash(messageID)
	targetIndex := hash % activeCount

	return subscriberIndex == targetIndex
}

// Hash simples para distribuição de mensagens
func (b *Balancer) simpleHash(s string) int {
	hash := 0
	for _, c := range s {
		hash = hash*31 + int(c)
	}
	if hash < 0 {
		hash = -hash
	}
	return hash
}

// GetActiveSubscribers retorna lista de subscribers ativos
func (b *Balancer) GetActiveSubscribers() []uuid.UUID {
	b.mutex.RLock()
	defer b.mutex.RUnlock()

	active := make([]uuid.UUID, 0)
	for id, sub := range b.subscribers {
		if sub.Active {
			active = append(active, id)
		}
	}
	return active
}
