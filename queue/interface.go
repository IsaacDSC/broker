package queue

// Queuer defines the interface for queue operations that provides thread-safe
// message queue functionality with support for claiming, processing tracking,
// and concurrent message handling.
//
// Example usage:
//
//	queue := NewQueueBase()
//	var q Queuer = queue
//
//	// Store messages
//	q.Store("user.created", map[string]any{"id": "123", "name": "John"})
//	q.Store("user.updated", map[string]any{"id": "456", "name": "Jane"})
//
//	// Load and process messages
//	if value, found := q.Load("user.created"); found {
//		// Process the message
//		fmt.Println("Processing:", value)
//	}
//
//	// Iterate over unclaimed messages
//	q.RangeUnclaimedMessages(func(messageID, key string, value any) bool {
//		if q.ClaimMessage(messageID) {
//			// Process the message
//			processMessage(key, value)
//			q.MarkAsProcessed(messageID)
//		}
//		return true // continue iteration
//	})
type Queuer interface {
	// Store adds a new message to the queue with the given key and value.
	// The message is automatically assigned a unique ID and marked as unclaimed.
	// This operation is thread-safe.
	//
	// Parameters:
	//   - key: The message key for grouping and retrieval
	//   - value: The message payload (can be any type)
	Store(key string, value any)

	// Load retrieves the first unclaimed message value by key.
	// Returns the value and true if found, nil and false otherwise.
	// This operation is thread-safe and only returns unclaimed messages.
	//
	// Parameters:
	//   - key: The message key to search for
	//
	// Returns:
	//   - value: The message payload if found
	//   - found: Boolean indicating if the message was found
	Load(key string) (any, bool)

	// Delete removes all messages with the given key from the queue.
	// This operation affects both claimed and unclaimed messages.
	// This operation is thread-safe.
	//
	// Parameters:
	//   - key: The message key to delete
	Delete(key string)

	// ClaimMessage attempts to claim a message for processing by marking it as claimed.
	// Once claimed, the message won't be returned by Load, Range, or other unclaimed
	// message operations. Returns true if successfully claimed, false if the message
	// doesn't exist or is already claimed. This operation is thread-safe.
	//
	// Parameters:
	//   - messageID: The unique ID of the message to claim
	//
	// Returns:
	//   - success: Boolean indicating if the message was successfully claimed
	ClaimMessage(messageID string) bool

	// MarkAsProcessed marks a message as processed and removes it from the main queue.
	// The message is moved to the processed tracking map and removed from active messages.
	// This operation is thread-safe.
	//
	// Parameters:
	//   - messageID: The unique ID of the message to mark as processed
	MarkAsProcessed(messageID string)

	// IsProcessed checks if a message has already been processed.
	// This operation is thread-safe.
	//
	// Parameters:
	//   - messageID: The unique ID of the message to check
	//
	// Returns:
	//   - processed: Boolean indicating if the message has been processed
	IsProcessed(messageID string) bool

	// GetByMaxConcurrency returns a new queue containing up to maxConcurrency
	// unclaimed messages from the current queue. The messages in the original
	// queue are marked as claimed to prevent double processing.
	// This operation is thread-safe.
	//
	// Parameters:
	//   - maxConcurrency: Maximum number of messages to include in the new queue
	//
	// Returns:
	//   - queue: A new Queuer instance containing the selected messages
	GetByMaxConcurrency(maxConcurrency int) Queuer

	// RangeUnclaimedMessages iterates over all unclaimed messages in the queue.
	// The iteration function receives the message ID, key, and value.
	// If the function returns false, iteration stops.
	// This operation creates a snapshot to avoid holding locks during iteration.
	//
	// Parameters:
	//   - fn: Function called for each unclaimed message
	//         Returns false to stop iteration, true to continue
	RangeUnclaimedMessages(fn func(messageID, key string, value any) bool)

	// Range iterates over all unclaimed messages in the queue.
	// The iteration function receives the message key and value.
	// If the function returns false, iteration stops.
	// This operation creates a snapshot to avoid holding locks during iteration.
	//
	// Parameters:
	//   - fn: Function called for each unclaimed message
	//         Returns false to stop iteration, true to continue
	Range(fn func(key string, value any) bool)

	// Len returns the number of unclaimed messages currently in the queue.
	// This operation is thread-safe.
	//
	// Returns:
	//   - count: Number of unclaimed messages in the queue
	Len() int

	// GetUnclaimedMessagesByKey returns all unclaimed messages for a specific key.
	// This operation is thread-safe and returns a slice of message pointers.
	//
	// Parameters:
	//   - key: The message key to search for
	//
	// Returns:
	//   - messages: Slice of Message pointers matching the key
	GetUnclaimedMessagesByKey(key string) []*Message
}
