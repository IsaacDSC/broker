package queue

import (
	"context"
	"fmt"
	"strings"
)

// USING SET
func (r *Redis) SetQueue(ctx context.Context, eventName string) error {
	key := strings.Join([]string{prefix, queuePrefix}, separator)
	pipe := r.client.Pipeline()
	pipe.SAdd(ctx, key, eventName)
	// Set TTL using maxRetention
	pipe.Expire(ctx, key, r.maxRetention)
	if _, err := pipe.Exec(ctx); err != nil {
		return fmt.Errorf("failed to set queue: %w", err)
	}
	return nil
}

// USING SET
func (r *Redis) GetQueues(ctx context.Context) ([]string, error) {
	key := strings.Join([]string{prefix, queuePrefix}, separator)
	return r.client.SMembers(ctx, key).Result()
}
