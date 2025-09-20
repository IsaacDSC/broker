package queue

import (
	"sync"

	"github.com/google/uuid"
)

// MemoryQueue implementação em memória da interface Queue
type MemoryQueue struct {
	messages       map[string]*Message
	mutex          sync.RWMutex
	processed      map[string]bool // Track processed messages
	processedMutex sync.RWMutex
	config         *QueueConfig
}

// NewMemoryQueue cria uma nova instância da fila em memória
func NewMemoryQueue(config *QueueConfig) *MemoryQueue {
	return &MemoryQueue{
		messages:  make(map[string]*Message),
		processed: make(map[string]bool),
		config:    config,
	}
}

func (mq *MemoryQueue) Store(key string, value any) error {
	mq.mutex.Lock()
	defer mq.mutex.Unlock()

	messageID := uuid.New().String()
	mq.messages[messageID] = &Message{
		ID:      messageID,
		Key:     key,
		Value:   value,
		Claimed: false,
	}
	return nil
}

func (mq *MemoryQueue) Load(key string) (any, bool) {
	mq.mutex.RLock()
	defer mq.mutex.RUnlock()

	for _, msg := range mq.messages {
		if msg.Key == key && !msg.Claimed {
			return msg.Value, true
		}
	}
	return nil, false
}

func (mq *MemoryQueue) Delete(key string) error {
	mq.mutex.Lock()
	defer mq.mutex.Unlock()

	for id, msg := range mq.messages {
		if msg.Key == key {
			delete(mq.messages, id)
			break
		}
	}
	return nil
}

func (mq *MemoryQueue) ClaimMessage(messageID string) bool {
	mq.mutex.Lock()
	defer mq.mutex.Unlock()

	msg, exists := mq.messages[messageID]
	if !exists || msg.Claimed {
		return false
	}

	msg.Claimed = true
	return true
}

func (mq *MemoryQueue) MarkAsProcessed(messageID string) error {
	mq.processedMutex.Lock()
	defer mq.processedMutex.Unlock()

	mq.processed[messageID] = true

	// Remove da fila principal
	mq.mutex.Lock()
	delete(mq.messages, messageID)
	mq.mutex.Unlock()

	return nil
}

func (mq *MemoryQueue) IsProcessed(messageID string) bool {
	mq.processedMutex.RLock()
	defer mq.processedMutex.RUnlock()

	return mq.processed[messageID]
}

func (mq *MemoryQueue) GetByMaxConcurrency(maxConcurrency int) Queue {
	mq.mutex.Lock()
	defer mq.mutex.Unlock()

	q := NewMemoryQueue(mq.config)
	counter := 0

	for id, msg := range mq.messages {
		if counter >= maxConcurrency {
			break
		}

		if !msg.Claimed {
			q.messages[id] = &Message{
				ID:      msg.ID,
				Key:     msg.Key,
				Value:   msg.Value,
				Claimed: false,
			}
			msg.Claimed = true
			counter++
		}
	}

	return q
}

func (mq *MemoryQueue) RangeUnclaimedMessages(fn func(messageID, key string, value any) bool) error {
	mq.mutex.RLock()
	messages := make(map[string]*Message)
	for id, msg := range mq.messages {
		if !msg.Claimed {
			messages[id] = msg
		}
	}
	mq.mutex.RUnlock()

	for id, msg := range messages {
		if !fn(id, msg.Key, msg.Value) {
			break
		}
	}
	return nil
}

func (mq *MemoryQueue) Range(fn func(key string, value any) bool) error {
	mq.mutex.RLock()
	messages := make(map[string]*Message)
	for _, msg := range mq.messages {
		if !msg.Claimed {
			messages[msg.ID] = msg
		}
	}
	mq.mutex.RUnlock()

	for _, msg := range messages {
		if !fn(msg.Key, msg.Value) {
			break
		}
	}
	return nil
}

func (mq *MemoryQueue) Len() (int, error) {
	mq.mutex.RLock()
	defer mq.mutex.RUnlock()

	count := 0
	for _, msg := range mq.messages {
		if !msg.Claimed {
			count++
		}
	}
	return count, nil
}

func (mq *MemoryQueue) GetUnclaimedMessagesByKey(key string) ([]*Message, error) {
	mq.mutex.RLock()
	defer mq.mutex.RUnlock()

	var messages []*Message
	for _, msg := range mq.messages {
		if msg.Key == key && !msg.Claimed {
			messages = append(messages, msg)
		}
	}
	return messages, nil
}

func (mq *MemoryQueue) Close() error {
	// Não há recursos para fechar na implementação em memória
	return nil
}

func (mq *MemoryQueue) Health() error {
	// Implementação em memória sempre está saudável
	return nil
}
