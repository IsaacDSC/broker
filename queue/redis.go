package queue

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/redis/go-redis/v9"
)

type Redis struct {
	client *redis.Client
	ctx    context.Context
}

const (
	prefix         = "gqueue"
	separator      = ":"
	queuePrefix    = "queue"
	messagesPrefix = "messages"
)

func NewRedis(client *redis.Client) *Redis {
	return &Redis{
		client: client,
		ctx:    context.Background(),
	}
}

const DefaultMaxMsg = -1

func (r *Redis) GetMsgsToProcess(ctx context.Context, eventName string, maxMsg int) ([]Msg, error) {
	if maxMsg == 0 {
		maxMsg = DefaultMaxMsg
	}
	if maxMsg == DefaultMaxMsg {
		maxMsg = 1000 // Set a reasonable limit
	}

	activeKey := r.createEventKey(eventName, ActiveStatus)
	processingKey := r.createEventKey(eventName, ProcessingStatus)

	var msgs []Msg

	// Use ZPOPMIN to atomically get and remove messages from active queue
	for i := 0; i < maxMsg; i++ {
		// ZPOPMIN atomically removes and returns the member with the lowest score
		result, err := r.client.ZPopMin(ctx, activeKey, 1).Result()
		if err != nil {
			if err == redis.Nil {
				break // No more messages
			}
			return nil, fmt.Errorf("failed to pop message from active queue: %w", err)
		}

		if len(result) == 0 {
			break // No more messages
		}

		// Parse the message
		msgStr := result[0].Member.(string)
		var msg Msg
		if err := json.Unmarshal([]byte(msgStr), &msg); err != nil {
			return nil, fmt.Errorf("failed to unmarshal message: %w", err)
		}

		// Update status to PROCESSING
		msg.Status = ProcessingStatus

		// Marshal the updated message
		updatedMsgBytes, err := json.Marshal(msg)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal updated message: %w", err)
		}

		// Add to processing queue with the same score
		score := result[0].Score
		if err := r.client.ZAdd(ctx, processingKey, redis.Z{
			Score:  score,
			Member: string(updatedMsgBytes),
		}).Err(); err != nil {
			return nil, fmt.Errorf("failed to add message to processing queue: %w", err)
		}

		msgs = append(msgs, msg)
	}

	return msgs, nil
}

var ErrNoMessagesFound = errors.New("no messages found")

// REDIS-CLI command to ZRANGEBYSCORE gqueue:queue:messages:eventName1.test_event:PROCESSING 1758844247677371 1758844247677371 WITHSCORES
func (r *Redis) ChangeStatus(ctx context.Context, eventName string, fromstatus, tostatus MsgQueueStatus, scores ...int64) error {
	start := scores[0]
	end := scores[len(scores)-1]

	sourceKey := r.createEventKey(eventName, fromstatus)
	results, err := r.client.ZRangeByScoreWithScores(ctx, sourceKey, &redis.ZRangeBy{
		Min: fmt.Sprintf("%d", start),
		Max: fmt.Sprintf("%d", end),
	}).Result()
	if err != nil {
		return fmt.Errorf("failed to get messages from event queue: %w", err)
	}

	if len(results) == 0 {
		return ErrNoMessagesFound
	}

	var membersToRemove []any
	targetKey := r.createEventKey(eventName, tostatus)

	for _, item := range results {
		msgStr := item.Member.(string)
		score := item.Score

		var msg Msg
		if err := json.Unmarshal([]byte(msgStr), &msg); err != nil {
			return fmt.Errorf("failed to unmarshal message: %w", err)
		} else {
			msg.Status = tostatus
		}

		msgBytes, err := json.Marshal(msg)
		if err != nil {
			return fmt.Errorf("failed to marshal message: %w", err)
		}

		if err := r.client.ZAdd(ctx, targetKey, redis.Z{
			Score:  score,
			Member: msgBytes,
		}).Err(); err != nil {
			return fmt.Errorf("failed to add message to target queue: %w", err)
		}

		membersToRemove = append(membersToRemove, msgStr)
	}

	// DELETE FROM SOURCE QUEUE
	if len(membersToRemove) > 0 {
		if err := r.client.ZRem(ctx, sourceKey, membersToRemove...).Err(); err != nil {
			return fmt.Errorf("failed to remove messages from source queue: %w", err)
		}
	}

	return nil
}
