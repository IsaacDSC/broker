package broker

import (
	"fmt"
	"sync"

	"github.com/google/uuid"
)

type QueueBase struct {
	sm sync.Map
}

func NewQueueBase() *QueueBase {
	return &QueueBase{
		sm: sync.Map{},
	}
}

func (qb *QueueBase) Store(key string, value any) {
	qb.sm.Store(key, value)
}

func (qb *QueueBase) Load(key string) (any, bool) {
	return qb.sm.Load(key)
}

func (qb *QueueBase) Delete(key string) {
	qb.sm.Delete(key)
}

func (qb *QueueBase) GetByMaxConcurrency(maxConcurrency int) *QueueBase {
	q := NewQueueBase()

	counter := 0
	qb.sm.Range(func(key, value interface{}) bool {

		if counter >= maxConcurrency {
			return false // para a iteração
		}

		keyStr := key.(string)
		q.Store(keyStr, value) //add to temp process queue
		qb.Delete(keyStr)      // delete from original queue
		counter++

		return true // continua a iteração
	})

	return q
}

func (qb *QueueBase) Range(fn func(key string, value any) bool) {
	qb.sm.Range(func(key, value interface{}) bool {
		keyStr := key.(string)
		return fn(keyStr, value)
	})
}

func (qb *QueueBase) Len() int {
	count := 0
	qb.sm.Range(func(key, value interface{}) bool {
		count++
		return true
	})
	return count
}

type Client struct {
	ID      uuid.UUID
	appName string
	// eventName > data
	Queue *QueueBase
}

func NewClient(appName string) *Client {
	return &Client{
		ID:      uuid.New(),
		appName: appName,
		Queue:   NewQueueBase(),
	}
}

func (b *Client) Publish(event string, payload any) error {
	b.Queue.Store(event, payload)
	return nil
}

func (b *Client) EventBrokerKey(eventName string) string {
	const brokerName = "gqueue"
	return fmt.Sprintf("%s:%s:%s", brokerName, b.appName, eventName)
}
