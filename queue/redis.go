package queue

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

type Redis struct {
	client redis.Cmdable
	ctx    context.Context
}

// redisMessage represents a message stored in Redis
type redisMessage struct {
	ID      string `json:"id"`
	Key     string `json:"key"`
	Value   any    `json:"value"`
	Claimed bool   `json:"claimed"`
}

const (
	// Redis keys
	messagesKey    = "queue:messages"  // Hash: messageID -> serialized message
	messagesByKeyK = "queue:keys"      // Hash: key -> messageID list (JSON array)
	claimedKey     = "queue:claimed"   // Set: claimed messageIDs
	processedKey   = "queue:processed" // Set: processed messageIDs
	unclaimedKey   = "queue:unclaimed" // Set: unclaimed messageIDs
)

func NewRedis(client redis.Cmdable) *Redis {
	return &Redis{
		client: client,
		ctx:    context.Background(),
	}
}

func NewRedisWithContext(client redis.Cmdable, ctx context.Context) *Redis {
	return &Redis{
		client: client,
		ctx:    ctx,
	}
}

// Store adds a new message to the queue with the given key and value
func (r *Redis) Store(key string, value any) {
	messageID := uuid.New().String()

	msg := redisMessage{
		ID:      messageID,
		Key:     key,
		Value:   value,
		Claimed: false,
	}

	// Serialize message
	msgBytes, err := json.Marshal(msg)
	if err != nil {
		panic(fmt.Sprintf("failed to serialize message: %v", err))
	}

	pipe := r.client.Pipeline()

	// Store message
	pipe.HSet(r.ctx, messagesKey, messageID, msgBytes)

	// Add to unclaimed set
	pipe.SAdd(r.ctx, unclaimedKey, messageID)

	// Add to key index
	pipe.SAdd(r.ctx, fmt.Sprintf("%s:%s", messagesByKeyK, key), messageID)

	_, err = pipe.Exec(r.ctx)
	if err != nil {
		panic(fmt.Sprintf("failed to store message: %v", err))
	}
}

// Load retrieves the first unclaimed message value by key
func (r *Redis) Load(key string) (any, bool) {
	// Get message IDs for this key
	messageIDs, err := r.client.SMembers(r.ctx, fmt.Sprintf("%s:%s", messagesByKeyK, key)).Result()
	if err != nil {
		return nil, false
	}

	if len(messageIDs) == 0 {
		return nil, false
	}

	// Find first unclaimed message
	for _, messageID := range messageIDs {
		// Check if unclaimed
		isUnclaimed, err := r.client.SIsMember(r.ctx, unclaimedKey, messageID).Result()
		if err != nil || !isUnclaimed {
			continue
		}

		// Get message
		msgBytes, err := r.client.HGet(r.ctx, messagesKey, messageID).Result()
		if err != nil {
			continue
		}

		var msg redisMessage
		if err := json.Unmarshal([]byte(msgBytes), &msg); err != nil {
			continue
		}

		return msg.Value, true
	}

	return nil, false
}

// Delete removes all messages with the given key from the queue
func (r *Redis) Delete(key string) {
	keySetName := fmt.Sprintf("%s:%s", messagesByKeyK, key)

	// Get all message IDs for this key
	messageIDs, err := r.client.SMembers(r.ctx, keySetName).Result()
	if err != nil {
		return
	}

	if len(messageIDs) == 0 {
		return
	}

	pipe := r.client.Pipeline()

	// Remove messages
	for _, messageID := range messageIDs {
		pipe.HDel(r.ctx, messagesKey, messageID)
		pipe.SRem(r.ctx, unclaimedKey, messageID)
		pipe.SRem(r.ctx, claimedKey, messageID)
		pipe.SRem(r.ctx, processedKey, messageID)
	}

	// Remove key index
	pipe.Del(r.ctx, keySetName)

	pipe.Exec(r.ctx)
}

// ClaimMessage attempts to claim a message for processing
func (r *Redis) ClaimMessage(messageID string) bool {
	// Check if message exists and is unclaimed
	exists, err := r.client.HExists(r.ctx, messagesKey, messageID).Result()
	if err != nil || !exists {
		return false
	}

	// Try to move from unclaimed to claimed atomically
	pipe := r.client.Pipeline()
	pipe.SRem(r.ctx, unclaimedKey, messageID)
	pipe.SAdd(r.ctx, claimedKey, messageID)

	results, err := pipe.Exec(r.ctx)
	if err != nil {
		return false
	}

	// Check if we successfully removed from unclaimed (meaning it was there)
	removeResult := results[0].(*redis.IntCmd)
	removed, err := removeResult.Result()
	if err != nil || removed == 0 {
		// Message was already claimed, rollback
		r.client.SRem(r.ctx, claimedKey, messageID)
		return false
	}

	return true
}

// MarkAsProcessed marks a message as processed and removes it from the main queue
func (r *Redis) MarkAsProcessed(messageID string) {
	pipe := r.client.Pipeline()

	// Get message to find its key
	msgBytes, err := r.client.HGet(r.ctx, messagesKey, messageID).Result()
	if err == nil {
		var msg redisMessage
		if json.Unmarshal([]byte(msgBytes), &msg) == nil {
			// Remove from key index
			pipe.SRem(r.ctx, fmt.Sprintf("%s:%s", messagesByKeyK, msg.Key), messageID)
		}
	}

	// Remove from all tracking sets and add to processed
	pipe.HDel(r.ctx, messagesKey, messageID)
	pipe.SRem(r.ctx, unclaimedKey, messageID)
	pipe.SRem(r.ctx, claimedKey, messageID)
	pipe.SAdd(r.ctx, processedKey, messageID)

	pipe.Exec(r.ctx)
}

// IsProcessed checks if a message has already been processed
func (r *Redis) IsProcessed(messageID string) bool {
	result, err := r.client.SIsMember(r.ctx, processedKey, messageID).Result()
	if err != nil {
		return false
	}
	return result
}

// GetByMaxConcurrency returns a new queue containing up to maxConcurrency unclaimed messages
func (r *Redis) GetByMaxConcurrency(maxConcurrency int) Queuer {
	// Create a new memory-based queue for isolation
	newQueue := NewQueueBase()

	// Get unclaimed message IDs from the original queue
	unclaimedIDs, err := r.client.SMembers(r.ctx, unclaimedKey).Result()
	if err != nil {
		return newQueue
	}

	count := 0
	for _, messageID := range unclaimedIDs {
		if count >= maxConcurrency {
			break
		}

		// Try to claim the message in the original queue
		if r.ClaimMessage(messageID) {
			// Get the message data
			msgBytes, err := r.client.HGet(r.ctx, messagesKey, messageID).Result()
			if err != nil {
				continue
			}

			var msg redisMessage
			if err := json.Unmarshal([]byte(msgBytes), &msg); err != nil {
				continue
			}

			// Store in new memory queue as unclaimed
			newQueue.Store(msg.Key, msg.Value)
			count++
		}
	}

	return newQueue
}

// RangeUnclaimedMessages iterates over all unclaimed messages in the queue
func (r *Redis) RangeUnclaimedMessages(fn func(messageID, key string, value any) bool) {
	// Get all unclaimed message IDs
	unclaimedIDs, err := r.client.SMembers(r.ctx, unclaimedKey).Result()
	if err != nil {
		return
	}

	for _, messageID := range unclaimedIDs {
		// Get message data
		msgBytes, err := r.client.HGet(r.ctx, messagesKey, messageID).Result()
		if err != nil {
			continue
		}

		var msg redisMessage
		if err := json.Unmarshal([]byte(msgBytes), &msg); err != nil {
			continue
		}

		// Call function with message data
		if !fn(messageID, msg.Key, msg.Value) {
			break
		}
	}
}

// Range iterates over all unclaimed messages in the queue
func (r *Redis) Range(fn func(key string, value any) bool) {
	r.RangeUnclaimedMessages(func(messageID, key string, value any) bool {
		return fn(key, value)
	})
}

// Len returns the number of unclaimed messages currently in the queue
func (r *Redis) Len() int {
	count, err := r.client.SCard(r.ctx, unclaimedKey).Result()
	if err != nil {
		return 0
	}
	return int(count)
}

// GetUnclaimedMessagesByKey returns all unclaimed messages for a specific key
func (r *Redis) GetUnclaimedMessagesByKey(key string) []*Message {
	var messages []*Message

	// Get message IDs for this key
	keySetName := fmt.Sprintf("%s:%s", messagesByKeyK, key)
	messageIDs, err := r.client.SMembers(r.ctx, keySetName).Result()
	if err != nil {
		return messages
	}

	for _, messageID := range messageIDs {
		// Check if unclaimed
		isUnclaimed, err := r.client.SIsMember(r.ctx, unclaimedKey, messageID).Result()
		if err != nil || !isUnclaimed {
			continue
		}

		// Get message data
		msgBytes, err := r.client.HGet(r.ctx, messagesKey, messageID).Result()
		if err != nil {
			continue
		}

		var msg redisMessage
		if err := json.Unmarshal([]byte(msgBytes), &msg); err != nil {
			continue
		}

		messages = append(messages, &Message{
			ID:      msg.ID,
			Key:     msg.Key,
			Value:   msg.Value,
			Claimed: false, // We only return unclaimed messages
		})
	}

	return messages
}

// Compile-time check to ensure Redis implements Queuer interface
var _ Queuer = (*Redis)(nil)
