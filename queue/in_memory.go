package queue

import (
	"sync"

	"github.com/google/uuid"
)

type Message struct {
	ID      string
	Key     string
	Value   any
	Claimed bool
}

type QueueBase struct {
	messages       map[string]*Message
	mutex          sync.RWMutex
	processed      map[string]bool // Track processed messages
	processedMutex sync.RWMutex
}

func NewQueueBase() *QueueBase {
	return &QueueBase{
		messages:  make(map[string]*Message),
		processed: make(map[string]bool),
	}
}

func (qb *QueueBase) Store(key string, value any) {
	qb.mutex.Lock()
	defer qb.mutex.Unlock()

	messageID := uuid.New().String()
	qb.messages[messageID] = &Message{
		ID:      messageID,
		Key:     key,
		Value:   value,
		Claimed: false,
	}
}

func (qb *QueueBase) Load(key string) (any, bool) {
	qb.mutex.RLock()
	defer qb.mutex.RUnlock()

	for _, msg := range qb.messages {
		if msg.Key == key && !msg.Claimed {
			return msg.Value, true
		}
	}
	return nil, false
}

func (qb *QueueBase) Delete(key string) {
	qb.mutex.Lock()
	defer qb.mutex.Unlock()

	for id, msg := range qb.messages {
		if msg.Key == key {
			delete(qb.messages, id)
			break
		}
	}
}

// ClaimMessage tenta reivindicar uma mensagem para processamento
func (qb *QueueBase) ClaimMessage(messageID string) bool {
	qb.mutex.Lock()
	defer qb.mutex.Unlock()

	msg, exists := qb.messages[messageID]
	if !exists || msg.Claimed {
		return false
	}

	msg.Claimed = true
	return true
}

// MarkAsProcessed marca uma mensagem como processada
func (qb *QueueBase) MarkAsProcessed(messageID string) {
	qb.processedMutex.Lock()
	defer qb.processedMutex.Unlock()

	qb.processed[messageID] = true

	// Remove da fila principal
	qb.mutex.Lock()
	delete(qb.messages, messageID)
	qb.mutex.Unlock()
}

// IsProcessed verifica se uma mensagem já foi processada
func (qb *QueueBase) IsProcessed(messageID string) bool {
	qb.processedMutex.RLock()
	defer qb.processedMutex.RUnlock()

	return qb.processed[messageID]
}

func (qb *QueueBase) GetByMaxConcurrency(maxConcurrency int) *QueueBase {
	qb.mutex.Lock()
	defer qb.mutex.Unlock()

	q := NewQueueBase()
	counter := 0

	for id, msg := range qb.messages {
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

// RangeUnclaimedMessages itera sobre mensagens não reivindicadas
func (qb *QueueBase) RangeUnclaimedMessages(fn func(messageID, key string, value any) bool) {
	qb.mutex.RLock()
	messages := make(map[string]*Message)
	for id, msg := range qb.messages {
		if !msg.Claimed {
			messages[id] = msg
		}
	}
	qb.mutex.RUnlock()

	for id, msg := range messages {
		if !fn(id, msg.Key, msg.Value) {
			break
		}
	}
}

func (qb *QueueBase) Range(fn func(key string, value any) bool) {
	qb.mutex.RLock()
	messages := make(map[string]*Message)
	for _, msg := range qb.messages {
		if !msg.Claimed {
			messages[msg.ID] = msg
		}
	}
	qb.mutex.RUnlock()

	for _, msg := range messages {
		if !fn(msg.Key, msg.Value) {
			break
		}
	}
}

func (qb *QueueBase) Len() int {
	qb.mutex.RLock()
	defer qb.mutex.RUnlock()

	count := 0
	for _, msg := range qb.messages {
		if !msg.Claimed {
			count++
		}
	}
	return count
}

// GetUnclaimedMessagesByKey retorna mensagens não reivindicadas por chave específica
func (qb *QueueBase) GetUnclaimedMessagesByKey(key string) []*Message {
	qb.mutex.RLock()
	defer qb.mutex.RUnlock()

	var messages []*Message
	for _, msg := range qb.messages {
		if msg.Key == key && !msg.Claimed {
			messages = append(messages, msg)
		}
	}
	return messages
}
