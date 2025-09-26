package queue

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

func (r *Redis) SaveEventMsg(ctx context.Context, eventName string, msg any, status MsgQueueStatus, unique ...bool) error {
	pipe := r.client.Pipeline()
	key := strings.Join([]string{prefix, eventName}, separator)

	// USING SET
	if !pipe.SIsMember(ctx, key, eventName).Val() {
		log.Default().Println("[*] Queue not found")
		// Se não existir registro deveriamos criar essa fila para aparecer no dashboard
		if err := r.SetQueue(ctx, eventName); err != nil {
			return fmt.Errorf("failed to sent msg: %w", err)
		}
	}

	timeNow := time.Now().UTC()
	var score float64
	if len(unique) > 0 && unique[0] {
		score = float64(timeNow.Hour())
		msg = Msg{
			ID:     fmt.Sprintf("unique:%f", score),
			Value:  msg,
			Time:   time.Date(timeNow.Year(), timeNow.Month(), timeNow.Day(), timeNow.Hour(), 0, 0, 0, timeNow.Location()),
			Status: status,
		}
	} else {
		score = float64(timeNow.UnixMicro())
		msg = Msg{
			ID:     uuid.NewString(),
			Value:  msg,
			Time:   timeNow,
			Status: status,
		}
	}

	msgInBytes, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}

	key = r.createEventKey(eventName, status)

	// USING SORTED SET
	pipe.ZAdd(ctx, key, redis.Z{
		Score:  score,
		Member: msgInBytes,
	})

	if _, err := pipe.Exec(ctx); err != nil {
		return fmt.Errorf("failed to save event message: %w", err)
	}

	return nil
}

func (r *Redis) GetMsgsByQueue(ctx context.Context, eventName string, status MsgQueueStatus, start int64, end int64) ([]Msg, error) {
	key := r.createEventKey(eventName, status)

	msgs, err := r.client.ZRange(ctx, key, start, end).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to get messages by queue: %w", err)
	}

	msgsList := make([]Msg, len(msgs))
	for i, msg := range msgs {
		var msgObj Msg
		if err := json.Unmarshal([]byte(msg), &msgObj); err != nil {
			return nil, fmt.Errorf("failed to unmarshal message: %w", err)
		}
		msgsList[i] = msgObj
	}
	return msgsList, nil
}

func (r *Redis) DeleteMsgsByQueue(ctx context.Context, eventName string, status MsgQueueStatus, start int64, end int64) error {
	key := r.createEventKey(eventName, status)

	if _, err := r.client.ZRemRangeByScore(ctx, key, fmt.Sprintf("%d", start), fmt.Sprintf("%d", end)).Result(); err != nil {
		return fmt.Errorf("failed to delete messages by queue: %w", err)
	}

	return nil
}

func (r *Redis) createEventKey(eventName string, status MsgQueueStatus) string {
	return strings.Join([]string{prefix, queuePrefix, messagesPrefix, eventName, status.String()}, separator)
}
