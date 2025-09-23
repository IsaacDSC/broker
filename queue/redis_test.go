package queue_test

import (
	"context"
	"testing"

	"github.com/IsaacDSC/broker/queue"
	"github.com/redis/go-redis/v9"
)

// setupRedisTest creates a Redis client for testing
func setupRedisTest(t *testing.T) (*redis.Client, func()) {
	// Use Redis database 15 for testing to avoid conflicts
	client := redis.NewClient(&redis.Options{
		Addr:     "localhost:6379",
		Password: "",
		DB:       15,
	})

	ctx := context.Background()

	// Test connection
	_, err := client.Ping(ctx).Result()
	if err != nil {
		t.Skipf("Redis not available: %v", err)
	}

	// Clean up function
	cleanup := func() {
		client.FlushDB(ctx)
		client.Close()
	}

	// Clear the test database
	client.FlushDB(ctx)

	return client, cleanup
}

// TestRedisQueue_basicUsage tests basic Redis queue operations
func TestRedisQueue_basicUsage(t *testing.T) {
	client, cleanup := setupRedisTest(t)
	defer cleanup()

	// Create Redis queue
	redisQueue := queue.NewRedis(client)
	var queuer queue.Queuer = redisQueue

	// Store some messages
	queuer.Store("user.created", map[string]any{
		"id":    "123",
		"name":  "John Doe",
		"email": "john@example.com",
	})

	queuer.Store("user.updated", map[string]any{
		"id":    "456",
		"name":  "Jane Smith",
		"email": "jane@example.com",
	})

	queuer.Store("user.created", map[string]any{
		"id":    "789",
		"name":  "Bob Johnson",
		"email": "bob@example.com",
	})

	// Check queue length
	if queuer.Len() != 3 {
		t.Errorf("Expected queue length 3, got %d", queuer.Len())
	}

	// Load a specific message by key
	if value, found := queuer.Load("user.updated"); found {
		userMap, ok := value.(map[string]any)
		if !ok {
			t.Errorf("Expected map[string]any, got %T", value)
		} else if userMap["name"] != "Jane Smith" {
			t.Errorf("Expected Jane Smith, got %v", userMap["name"])
		}
	} else {
		t.Error("Expected to find user.updated message")
	}

	// Iterate over all unclaimed messages and count them
	messageCount := 0
	userCreatedCount := 0
	userUpdatedCount := 0

	queuer.Range(func(key string, value any) bool {
		messageCount++
		if key == "user.created" {
			userCreatedCount++
		} else if key == "user.updated" {
			userUpdatedCount++
		}
		return true // continue iteration
	})

	if messageCount != 3 {
		t.Errorf("Expected 3 messages in range, got %d", messageCount)
	}
	if userCreatedCount != 2 {
		t.Errorf("Expected 2 user.created messages, got %d", userCreatedCount)
	}
	if userUpdatedCount != 1 {
		t.Errorf("Expected 1 user.updated message, got %d", userUpdatedCount)
	}
}

// TestRedisQueue_messageProcessing tests message claiming and processing
func TestRedisQueue_messageProcessing(t *testing.T) {
	client, cleanup := setupRedisTest(t)
	defer cleanup()

	redisQueue := queue.NewRedis(client)
	var queuer queue.Queuer = redisQueue

	// Store some messages
	queuer.Store("task.process", "Task 1")
	queuer.Store("task.process", "Task 2")
	queuer.Store("task.process", "Task 3")

	if queuer.Len() != 3 {
		t.Errorf("Expected initial queue length 3, got %d", queuer.Len())
	}

	// Process messages by claiming them
	processedCount := 0
	queuer.RangeUnclaimedMessages(func(messageID, key string, value any) bool {
		// Try to claim the message
		if queuer.ClaimMessage(messageID) {
			t.Logf("Claimed and processing: %s = %v", key, value)

			// Mark as processed when done
			queuer.MarkAsProcessed(messageID)
			processedCount++

			// Check if it's marked as processed
			if !queuer.IsProcessed(messageID) {
				t.Errorf("Message %s should be marked as processed", messageID)
			}
		}
		return true // continue iteration
	})

	if processedCount != 3 {
		t.Errorf("Expected to process 3 messages, got %d", processedCount)
	}

	if queuer.Len() != 0 {
		t.Errorf("Expected final queue length 0, got %d", queuer.Len())
	}
}

// TestRedisQueue_concurrency tests concurrent message processing
func TestRedisQueue_concurrency(t *testing.T) {
	client, cleanup := setupRedisTest(t)
	defer cleanup()

	redisQueue := queue.NewRedis(client)
	var queuer queue.Queuer = redisQueue

	// Store multiple messages
	for i := 1; i <= 10; i++ {
		queuer.Store("work.item", map[string]any{
			"id":   i,
			"task": "Work item " + string(rune(i)),
		})
	}

	if queuer.Len() != 10 {
		t.Errorf("Expected total messages 10, got %d", queuer.Len())
	}

	// Process with max concurrency of 3
	batch := queuer.GetByMaxConcurrency(3)

	if batch.Len() != 3 {
		t.Errorf("Expected batch size 3, got %d", batch.Len())
	}

	// Original queue should have 7 remaining (3 were claimed)
	if queuer.Len() != 7 {
		t.Errorf("Expected remaining in original queue 7, got %d", queuer.Len())
	}

	// Process the batch
	processedCount := 0
	batch.RangeUnclaimedMessages(func(messageID, key string, value any) bool {
		t.Logf("Processing: %s = %v", key, value)
		batch.MarkAsProcessed(messageID)
		processedCount++
		return true
	})

	if batch.Len() != 0 {
		t.Errorf("Expected batch length 0 after processing, got %d", batch.Len())
	}

	if processedCount != 3 {
		t.Errorf("Expected to process 3 messages, got %d", processedCount)
	}
}

// TestRedisQueue_keyFiltering tests working with specific message keys
func TestRedisQueue_keyFiltering(t *testing.T) {
	client, cleanup := setupRedisTest(t)
	defer cleanup()

	redisQueue := queue.NewRedis(client)
	var queuer queue.Queuer = redisQueue

	// Store messages with different keys
	queuer.Store("email.send", map[string]any{"to": "user1@example.com", "subject": "Welcome"})
	queuer.Store("email.send", map[string]any{"to": "user2@example.com", "subject": "Newsletter"})
	queuer.Store("sms.send", map[string]any{"to": "+1234567890", "message": "Hello"})
	queuer.Store("email.send", map[string]any{"to": "user3@example.com", "subject": "Update"})

	if queuer.Len() != 4 {
		t.Errorf("Expected total messages 4, got %d", queuer.Len())
	}

	// Get all email messages
	emailMessages := queuer.GetUnclaimedMessagesByKey("email.send")
	if len(emailMessages) != 3 {
		t.Errorf("Expected email messages count 3, got %d", len(emailMessages))
	}

	// Process only email messages
	processedEmails := 0
	for _, msg := range emailMessages {
		if queuer.ClaimMessage(msg.ID) {
			t.Logf("Processing email: %v", msg.Value)
			queuer.MarkAsProcessed(msg.ID)
			processedEmails++
		}
	}

	if processedEmails != 3 {
		t.Errorf("Expected to process 3 emails, got %d", processedEmails)
	}

	if queuer.Len() != 1 {
		t.Errorf("Expected remaining messages 1, got %d", queuer.Len())
	}

	// Delete all SMS messages
	queuer.Delete("sms.send")
	if queuer.Len() != 0 {
		t.Errorf("Expected 0 messages after deleting SMS, got %d", queuer.Len())
	}
}

// TestRedisQueue_claimRace tests claiming race conditions
func TestRedisQueue_claimRace(t *testing.T) {
	client, cleanup := setupRedisTest(t)
	defer cleanup()

	redisQueue := queue.NewRedis(client)
	var queuer queue.Queuer = redisQueue

	// Store a message
	queuer.Store("test.claim", "test value")

	var messageID string
	queuer.RangeUnclaimedMessages(func(id, key string, value any) bool {
		messageID = id
		return false // stop iteration
	})

	// First claim should succeed
	if !queuer.ClaimMessage(messageID) {
		t.Error("First claim should succeed")
	}

	// Second claim should fail
	if queuer.ClaimMessage(messageID) {
		t.Error("Second claim should fail")
	}

	// Message should not be available for Load
	if _, found := queuer.Load("test.claim"); found {
		t.Error("Claimed message should not be available for Load")
	}
}

// TestRedisQueue_persistence tests that data persists across Redis queue instances
func TestRedisQueue_persistence(t *testing.T) {
	client, cleanup := setupRedisTest(t)
	defer cleanup()

	// Create first queue instance and store data
	{
		redisQueue1 := queue.NewRedis(client)
		redisQueue1.Store("persist.test", "persistent value")

		if redisQueue1.Len() != 1 {
			t.Errorf("Expected queue length 1, got %d", redisQueue1.Len())
		}
	}

	// Create second queue instance and verify data is still there
	{
		redisQueue2 := queue.NewRedis(client)

		if redisQueue2.Len() != 1 {
			t.Errorf("Expected queue length 1 after reconnect, got %d", redisQueue2.Len())
		}

		if value, found := redisQueue2.Load("persist.test"); !found {
			t.Error("Expected to find persistent message")
		} else if value != "persistent value" {
			t.Errorf("Expected 'persistent value', got %v", value)
		}
	}
}

// TestRedisQueue_deleteNonExistent tests deleting non-existent keys
func TestRedisQueue_deleteNonExistent(t *testing.T) {
	client, cleanup := setupRedisTest(t)
	defer cleanup()

	redisQueue := queue.NewRedis(client)
	var queuer queue.Queuer = redisQueue

	// Store some messages
	queuer.Store("existing.key", "value")

	// Delete non-existent key (should not panic)
	queuer.Delete("non.existent.key")

	// Original message should still be there
	if queuer.Len() != 1 {
		t.Errorf("Expected queue length 1, got %d", queuer.Len())
	}
}

// TestRedisQueue_emptyOperations tests operations on empty queue
func TestRedisQueue_emptyOperations(t *testing.T) {
	client, cleanup := setupRedisTest(t)
	defer cleanup()

	redisQueue := queue.NewRedis(client)
	var queuer queue.Queuer = redisQueue

	// Test empty queue operations
	if queuer.Len() != 0 {
		t.Errorf("Expected empty queue length 0, got %d", queuer.Len())
	}

	if value, found := queuer.Load("nonexistent"); found {
		t.Errorf("Expected not to find value, got %v", value)
	}

	if queuer.ClaimMessage("nonexistent-id") {
		t.Error("Expected claim to fail for nonexistent message")
	}

	if queuer.IsProcessed("nonexistent-id") {
		t.Error("Expected IsProcessed to return false for nonexistent message")
	}

	messages := queuer.GetUnclaimedMessagesByKey("nonexistent")
	if len(messages) != 0 {
		t.Errorf("Expected no messages, got %d", len(messages))
	}

	batch := queuer.GetByMaxConcurrency(5)
	if batch.Len() != 0 {
		t.Errorf("Expected empty batch, got length %d", batch.Len())
	}

	// Range operations should not call function
	queuer.Range(func(key string, value any) bool {
		t.Error("Range function should not be called on empty queue")
		return false
	})

	queuer.RangeUnclaimedMessages(func(messageID, key string, value any) bool {
		t.Error("RangeUnclaimedMessages function should not be called on empty queue")
		return false
	})
}

// BenchmarkRedisQueue_Store benchmarks the Redis Store operation
func BenchmarkRedisQueue_Store(b *testing.B) {
	client := redis.NewClient(&redis.Options{
		Addr:     "localhost:6379",
		Password: "",
		DB:       15,
	})
	defer client.Close()

	ctx := context.Background()
	if _, err := client.Ping(ctx).Result(); err != nil {
		b.Skipf("Redis not available: %v", err)
	}

	client.FlushDB(ctx)
	redisQueue := queue.NewRedis(client)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		redisQueue.Store("benchmark", i)
	}
}

// BenchmarkRedisQueue_Load benchmarks the Redis Load operation
func BenchmarkRedisQueue_Load(b *testing.B) {
	client := redis.NewClient(&redis.Options{
		Addr:     "localhost:6379",
		Password: "",
		DB:       15,
	})
	defer client.Close()

	ctx := context.Background()
	if _, err := client.Ping(ctx).Result(); err != nil {
		b.Skipf("Redis not available: %v", err)
	}

	client.FlushDB(ctx)
	redisQueue := queue.NewRedis(client)

	// Pre-populate with some data
	for i := 0; i < 1000; i++ {
		redisQueue.Store("benchmark", i)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		redisQueue.Load("benchmark")
	}
}
