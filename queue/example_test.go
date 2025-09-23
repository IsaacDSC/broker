package queue_test

import (
	"fmt"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/IsaacDSC/broker/queue"
)

// TestQueuer_basicUsage demonstrates basic usage of the Queuer interface
func TestQueuer_basicUsage(t *testing.T) {
	// Create a new queue instance
	q := queue.NewQueueBase()

	// Use the interface
	var queuer queue.Queuer = q

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

// TestQueuer_messageProcessing demonstrates message claiming and processing
func TestQueuer_messageProcessing(t *testing.T) {
	q := queue.NewQueueBase()
	var queuer queue.Queuer = q

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

			// Simulate processing work
			// In real scenarios, you would do actual work here

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

// TestQueuer_concurrency demonstrates concurrent message processing
func TestQueuer_concurrency(t *testing.T) {
	q := queue.NewQueueBase()
	var queuer queue.Queuer = q

	// Store multiple messages
	for i := 1; i <= 10; i++ {
		queuer.Store("work.item", fmt.Sprintf("Work item %d", i))
	}

	if queuer.Len() != 10 {
		t.Errorf("Expected total messages 10, got %d", queuer.Len())
	}

	// Process with max concurrency of 3
	batch := queuer.GetByMaxConcurrency(3)

	if batch.Len() != 3 {
		t.Errorf("Expected batch size 3, got %d", batch.Len())
	}

	if queuer.Len() != 7 {
		t.Errorf("Expected remaining in original queue 7, got %d", queuer.Len())
	}

	// Process the batch
	var wg sync.WaitGroup
	var processedCount int64
	batch.RangeUnclaimedMessages(func(messageID, key string, value any) bool {
		wg.Add(1)
		go func(id, k string, v any) {
			defer wg.Done()
			t.Logf("Processing in goroutine: %s = %v", k, v)
			batch.MarkAsProcessed(id)
			atomic.AddInt64(&processedCount, 1)
		}(messageID, key, value)
		return true
	})

	wg.Wait()

	if batch.Len() != 0 {
		t.Errorf("Expected batch length 0 after processing, got %d", batch.Len())
	}

	if atomic.LoadInt64(&processedCount) != 3 {
		t.Errorf("Expected to process 3 messages, got %d", atomic.LoadInt64(&processedCount))
	}
}

// TestQueuer_keyFiltering demonstrates working with specific message keys
func TestQueuer_keyFiltering(t *testing.T) {
	q := queue.NewQueueBase()
	var queuer queue.Queuer = q

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

// TestQueuerInterface ensures that QueueBase implements Queuer
func TestQueuerInterface(t *testing.T) {
	var _ queue.Queuer = (*queue.QueueBase)(nil)

	// Test that we can use QueueBase as Queuer
	q := queue.NewQueueBase()
	var queuer queue.Queuer = q

	// Basic functionality test
	queuer.Store("test", "value")
	if queuer.Len() != 1 {
		t.Errorf("Expected queue length 1, got %d", queuer.Len())
	}

	value, found := queuer.Load("test")
	if !found || value != "value" {
		t.Errorf("Expected to find 'value', got %v (found: %v)", value, found)
	}
}

// BenchmarkQueuer_Store benchmarks the Store operation
func BenchmarkQueuer_Store(b *testing.B) {
	q := queue.NewQueueBase()
	var queuer queue.Queuer = q

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		queuer.Store("benchmark", i)
	}
}

// BenchmarkQueuer_Load benchmarks the Load operation
func BenchmarkQueuer_Load(b *testing.B) {
	q := queue.NewQueueBase()
	var queuer queue.Queuer = q

	// Pre-populate with some data
	for i := 0; i < 1000; i++ {
		queuer.Store("benchmark", i)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		queuer.Load("benchmark")
	}
}
