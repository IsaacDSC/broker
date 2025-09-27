package queue

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/testcontainers/testcontainers-go"
	rediscontainer "github.com/testcontainers/testcontainers-go/modules/redis"
)

// setupRedisContainer creates a Redis testcontainer and returns the client
func setupRedisContainer(t *testing.T) (client *redis.Client, cleanup func()) {
	// First try to connect to local Redis instance
	if localClient := tryLocalRedis(t); localClient != nil {
		return localClient, func() { localClient.Close() }
	}

	// Fallback to testcontainer
	defer func() {
		if r := recover(); r != nil {
			t.Skipf("Docker panic recovered: %v", r)
			client = nil
			cleanup = func() {}
		}
	}()

	ctx := context.Background()

	// Try to start Redis container with better error handling
	redisContainer, err := rediscontainer.RunContainer(ctx,
		testcontainers.WithImage("redis:7-alpine"),
	)
	if err != nil {
		t.Skipf("Docker not available or failed to start Redis container: %v", err)
		return nil, func() {}
	}

	endpoint, err := redisContainer.Endpoint(ctx, "")
	if err != nil {
		redisContainer.Terminate(ctx)
		t.Skipf("failed to get Redis endpoint: %v", err)
		return nil, func() {}
	}

	redisClient := redis.NewClient(&redis.Options{
		Addr: endpoint,
	})

	// Test connection with timeout
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	_, err = redisClient.Ping(ctx).Result()
	if err != nil {
		redisClient.Close()
		redisContainer.Terminate(context.Background())
		t.Skipf("failed to connect to Redis: %v", err)
		return nil, func() {}
	}

	cleanup = func() {
		redisClient.Close()
		redisContainer.Terminate(context.Background())
	}

	return redisClient, cleanup
}

// tryLocalRedis attempts to connect to a local Redis instance
func tryLocalRedis(t *testing.T) *redis.Client {
	localAddresses := []string{
		"localhost:6379",
		"localhost:6380",
		"127.0.0.1:6379",
		"127.0.0.1:6380",
		"redis:6379",
	}

	for _, addr := range localAddresses {
		client := redis.NewClient(&redis.Options{
			Addr: addr,
			DB:   1, // Use test database
		})

		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		_, err := client.Ping(ctx).Result()
		cancel()

		if err == nil {
			t.Logf("Using local Redis instance at %s", addr)
			// Clean the test database
			client.FlushDB(context.Background())
			return client
		}
		client.Close()
	}

	return nil
}

func TestSaveEventMsg(t *testing.T) {
	redisClient := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
	})

	ctx := context.Background()
	redisQueue := NewRedis(redisClient)

	assert.NoError(t, redisClient.Ping(ctx).Err())

	tests := []struct {
		name      string
		eventName string
		payload   map[string]any
		isUnique  bool
	}{
		{
			name:      uuid.New().String(),
			eventName: "eventName.test_event",
			payload:   map[string]any{"key": "value"},
		},
		{
			name:      uuid.New().String(),
			eventName: "eventName.test_event",
			payload:   map[string]any{"key": "value"},
			isUnique:  true,
		},
	}

	for _, tc := range tests {
		key := redisQueue.createEventKey(tc.eventName, ActiveStatus)
		redisClient.FlushDB(ctx)
		if tc.isUnique {
			assert.NoError(t, redisQueue.SaveEventMsg(ctx, tc.eventName, tc.payload, ActiveStatus, tc.isUnique))
			assert.NoError(t, redisQueue.SaveEventMsg(ctx, tc.eventName, tc.payload, ActiveStatus, tc.isUnique))
			assert.Equal(t, int64(1), redisClient.ZCard(ctx, key).Val())
		} else {
			assert.NoError(t, redisQueue.SaveEventMsg(ctx, tc.eventName, tc.payload, ActiveStatus, tc.isUnique))
			assert.NoError(t, redisQueue.SaveEventMsg(ctx, tc.eventName, tc.payload, ActiveStatus, tc.isUnique))
			assert.Equal(t, int64(2), redisClient.ZCard(ctx, key).Val())
		}

	}

}

func TestGetMsgsOnQueue(t *testing.T) {
	redisClient, cleanup := setupRedisContainer(t)
	if redisClient == nil {
		panic("redis client is nil")
	}

	defer cleanup()

	ctx := context.Background()
	redisClient.FlushDB(ctx)

	redisQueue := NewRedis(redisClient)

	seedData(ctx, redisQueue)

	eventsName, err := redisQueue.GetQueues(ctx)
	assert.NoError(t, err)
	expected := []string{"eventName1.test_event", "eventName2.test_event", "eventName3.test_event"}
	assert.ElementsMatch(t, expected, eventsName)

	for _, eventName := range expected {
		msgs, err := redisQueue.GetMsgsByQueue(ctx, eventName, ActiveStatus, 0, DefaultMaxMsg)
		assert.NoError(t, err)

		switch eventName {
		case "eventName1.test_event":
			assert.Equal(t, 2, len(msgs), "Expected 2 messages for eventName1.test_event")
		case "eventName2.test_event":
			assert.Equal(t, 2, len(msgs), "Expected 2 messages for eventName2.test_event")
		case "eventName3.test_event":
			assert.Equal(t, 1, len(msgs), "Expected 1 message for eventName3.test_event")
		}
	}
}

func TestGetMsgsWithConcurrency(t *testing.T) {
	redisClient := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
	})

	ctx := context.Background()
	redisQueue := NewRedis(redisClient)
	seedData(ctx, redisQueue)
	defer redisClient.FlushAll(ctx)

	eventsNames, err := redisQueue.GetQueues(ctx)
	assert.NoError(t, err)

	var expectedMsgs []Msg
	for _, eventName := range eventsNames {
		msgs, err := redisQueue.GetMsgsByQueue(ctx, eventName, ActiveStatus, 0, DefaultMaxMsg)
		assert.NoError(t, err)

		for _, msg := range msgs {
			msg.Status = ProcessingStatus
			expectedMsgs = append(expectedMsgs, msg)
		}
	}

	var wg sync.WaitGroup
	wg.Add(2)

	var result1 []Msg
	var result2 []Msg

	// CH1
	go func() {
		defer wg.Done()
		counter := 0
		for _, eventName := range eventsNames {
			msgs, err := redisQueue.GetMsgsToProcess(ctx, eventName, DefaultMaxMsg)
			assert.NoError(t, err)
			if len(msgs) == 0 {
				continue
			}
			result1 = append(result1, msgs...)
			counter++
		}
	}()

	// CH2
	go func() {
		defer wg.Done()
		counter := 0
		for _, eventName := range eventsNames {
			msgs, err := redisQueue.GetMsgsToProcess(ctx, eventName, DefaultMaxMsg)
			assert.NoError(t, err)
			if len(msgs) == 0 {
				continue
			}
			result2 = append(result2, msgs...)
			counter++
		}
	}()

	wg.Wait()
	var result []Msg
	result = append(result1, result2...)

	assert.Equal(t, len(result), len(expectedMsgs))
	assert.ElementsMatch(t, result, expectedMsgs)

}

func TestCreateSeed(t *testing.T) {
	redisClient, cleanup := setupRedisContainer(t)
	if redisClient == nil {
		t.Skip("Redis client is nil")
	}
	defer cleanup()

	ctx := context.Background()
	redisQueue := NewRedis(redisClient)
	seedData(ctx, redisQueue)
}

func seedData(ctx context.Context, redisQueue *Redis) {
	redisQueue.SaveEventMsg(ctx, "eventName1.test_event", map[string]any{"key": "value1"}, ActiveStatus, false)
	redisQueue.SaveEventMsg(ctx, "eventName1.test_event", map[string]any{"key": "value2"}, ActiveStatus, false)
	redisQueue.SaveEventMsg(ctx, "eventName2.test_event", map[string]any{"key": "value3"}, ActiveStatus, false)
	redisQueue.SaveEventMsg(ctx, "eventName2.test_event", map[string]any{"key": "value4"}, ActiveStatus, false)
	redisQueue.SaveEventMsg(ctx, "eventName3.test_event", map[string]any{"key": "value5"}, ActiveStatus, false)
}

func TestChangeStatus(t *testing.T) {
	redisClient := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
	})

	ctx := context.Background()
	redisQueue := NewRedis(redisClient)
	defer redisClient.FlushAll(ctx)

	fromStatus := ProcessingStatus
	toStatus := AckedStatus

	expectedMsg := map[string]any{"key": "value1"}
	timeNow := time.Now().UTC()
	msgData := Msg{
		ID:     uuid.New().String(),
		Value:  expectedMsg,
		Status: ProcessingStatus,
		Time:   timeNow,
	}

	b, _ := json.Marshal(msgData)
	eventName := fmt.Sprintf("eventName1.test_event.%s", time.Now().Format("20060102150405"))
	key := redisQueue.createEventKey(eventName, fromStatus)
	redisClient.ZAdd(ctx, key, redis.Z{
		Score:  float64(timeNow.UnixMicro()),
		Member: b,
	})

	assert.NoError(t, redisQueue.ChangeStatus(ctx, eventName, fromStatus, toStatus, timeNow.UnixMicro(), timeNow.UnixMicro()))

	processingMsgs, err := redisClient.ZRange(ctx, key, 0, -1).Result()
	assert.NoError(t, err)
	assert.Len(t, processingMsgs, 0)

	ackedKey := redisQueue.createEventKey(eventName, toStatus)
	ackedMsgs, err := redisClient.ZRange(ctx, ackedKey, 0, -1).Result()
	assert.NoError(t, err)
	assert.Len(t, ackedMsgs, 1)

	var msg Msg
	err = json.Unmarshal([]byte(ackedMsgs[0]), &msg)
	assert.NoError(t, err)
	assert.Equal(t, toStatus, msg.Status)
	assert.Equal(t, expectedMsg, msg.Value)

}
