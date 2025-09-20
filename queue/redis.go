package queue

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

// RedisQueue implementação Redis da interface Queue
type RedisQueue struct {
	client  *redis.Client
	config  *QueueConfig
	ctx     context.Context
	prefix  string
	lockTTL time.Duration
}

// NewRedisQueue cria uma nova instância da fila Redis
func NewRedisQueue(config *QueueConfig) (*RedisQueue, error) {
	if config.Redis == nil {
		return nil, fmt.Errorf("redis config is required")
	}

	// Configurações padrão
	redisConfig := config.Redis
	if redisConfig.PoolSize == 0 {
		redisConfig.PoolSize = 10
	}
	if redisConfig.MaxRetries == 0 {
		redisConfig.MaxRetries = 3
	}
	if redisConfig.MinIdleConns == 0 {
		redisConfig.MinIdleConns = 2
	}

	client := redis.NewClient(&redis.Options{
		Addr:         redisConfig.Addr,
		Password:     redisConfig.Password,
		DB:           redisConfig.DB,
		PoolSize:     redisConfig.PoolSize,
		MaxRetries:   redisConfig.MaxRetries,
		MinIdleConns: redisConfig.MinIdleConns,
	})

	ctx := context.Background()

	// Testa a conexão
	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to connect to redis: %w", err)
	}

	prefix := "queue"
	if config.Prefix != "" {
		prefix = config.Prefix
	}
	if config.AppName != "" {
		prefix = fmt.Sprintf("%s:%s", prefix, config.AppName)
	}

	return &RedisQueue{
		client:  client,
		config:  config,
		ctx:     ctx,
		prefix:  prefix,
		lockTTL: 30 * time.Second, // TTL padrão para locks
	}, nil
}

func (rq *RedisQueue) getMessageKey(messageID string) string {
	return fmt.Sprintf("%s:messages:%s", rq.prefix, messageID)
}

func (rq *RedisQueue) getQueueKey(eventKey string) string {
	return fmt.Sprintf("%s:events:%s", rq.prefix, eventKey)
}

func (rq *RedisQueue) getProcessedKey(messageID string) string {
	return fmt.Sprintf("%s:processed:%s", rq.prefix, messageID)
}

func (rq *RedisQueue) getClaimKey(messageID string) string {
	return fmt.Sprintf("%s:claims:%s", rq.prefix, messageID)
}

func (rq *RedisQueue) Store(key string, value any) error {
	messageID := uuid.New().String()

	message := &Message{
		ID:      messageID,
		Key:     key,
		Value:   value,
		Claimed: false,
	}

	messageData, err := json.Marshal(message)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}

	pipe := rq.client.Pipeline()

	// Armazena a mensagem
	pipe.Set(rq.ctx, rq.getMessageKey(messageID), messageData, 0)

	// Adiciona à lista do evento
	pipe.LPush(rq.ctx, rq.getQueueKey(key), messageID)

	_, err = pipe.Exec(rq.ctx)
	if err != nil {
		return fmt.Errorf("failed to store message: %w", err)
	}

	return nil
}

func (rq *RedisQueue) Load(key string) (any, bool) {
	messageIDs, err := rq.client.LRange(rq.ctx, rq.getQueueKey(key), 0, -1).Result()
	if err != nil {
		return nil, false
	}

	for _, messageID := range messageIDs {
		// Verifica se a mensagem não está reivindicada
		claimed, err := rq.client.Exists(rq.ctx, rq.getClaimKey(messageID)).Result()
		if err != nil || claimed > 0 {
			continue
		}

		messageData, err := rq.client.Get(rq.ctx, rq.getMessageKey(messageID)).Result()
		if err != nil {
			continue
		}

		var message Message
		if err := json.Unmarshal([]byte(messageData), &message); err != nil {
			continue
		}

		return message.Value, true
	}

	return nil, false
}

func (rq *RedisQueue) Delete(key string) error {
	messageIDs, err := rq.client.LRange(rq.ctx, rq.getQueueKey(key), 0, -1).Result()
	if err != nil {
		return err
	}

	if len(messageIDs) == 0 {
		return nil
	}

	pipe := rq.client.Pipeline()

	for _, messageID := range messageIDs {
		pipe.Del(rq.ctx, rq.getMessageKey(messageID))
		pipe.Del(rq.ctx, rq.getClaimKey(messageID))
		pipe.Del(rq.ctx, rq.getProcessedKey(messageID))
	}

	pipe.Del(rq.ctx, rq.getQueueKey(key))

	_, err = pipe.Exec(rq.ctx)
	return err
}

func (rq *RedisQueue) ClaimMessage(messageID string) bool {
	// Tenta criar um lock atômico com TTL
	result, err := rq.client.SetNX(rq.ctx, rq.getClaimKey(messageID), time.Now().Unix(), rq.lockTTL).Result()
	if err != nil {
		return false
	}

	return result
}

func (rq *RedisQueue) MarkAsProcessed(messageID string) error {
	pipe := rq.client.Pipeline()

	// Marca como processado
	pipe.Set(rq.ctx, rq.getProcessedKey(messageID), time.Now().Unix(), 24*time.Hour)

	// Remove a mensagem e claim
	pipe.Del(rq.ctx, rq.getMessageKey(messageID))
	pipe.Del(rq.ctx, rq.getClaimKey(messageID))

	// Remove das listas de eventos
	pipe.Eval(rq.ctx, `
		local messageID = ARGV[1]
		local prefix = ARGV[2]
		local keys = redis.call('KEYS', prefix .. ':events:*')
		for i=1,#keys do
			redis.call('LREM', keys[i], 0, messageID)
		end
		return #keys
	`, []string{}, messageID, rq.prefix)

	_, err := pipe.Exec(rq.ctx)
	return err
}

func (rq *RedisQueue) IsProcessed(messageID string) bool {
	exists, err := rq.client.Exists(rq.ctx, rq.getProcessedKey(messageID)).Result()
	if err != nil {
		return false
	}
	return exists > 0
}

func (rq *RedisQueue) GetByMaxConcurrency(maxConcurrency int) Queue {
	// Para Redis, retornamos a mesma instância pois o controle é feito via claims
	return rq
}

func (rq *RedisQueue) RangeUnclaimedMessages(fn func(messageID, key string, value any) bool) error {
	// Busca todas as chaves de eventos
	eventKeys, err := rq.client.Keys(rq.ctx, rq.getQueueKey("*")).Result()
	if err != nil {
		return err
	}

	for _, eventKey := range eventKeys {
		messageIDs, err := rq.client.LRange(rq.ctx, eventKey, 0, -1).Result()
		if err != nil {
			continue
		}

		for _, messageID := range messageIDs {
			// Verifica se não está reivindicada
			claimed, err := rq.client.Exists(rq.ctx, rq.getClaimKey(messageID)).Result()
			if err != nil || claimed > 0 {
				continue
			}

			// Verifica se não foi processada
			if rq.IsProcessed(messageID) {
				continue
			}

			messageData, err := rq.client.Get(rq.ctx, rq.getMessageKey(messageID)).Result()
			if err != nil {
				continue
			}

			var message Message
			if err := json.Unmarshal([]byte(messageData), &message); err != nil {
				continue
			}

			if !fn(messageID, message.Key, message.Value) {
				return nil
			}
		}
	}

	return nil
}

func (rq *RedisQueue) Range(fn func(key string, value any) bool) error {
	return rq.RangeUnclaimedMessages(func(messageID, key string, value any) bool {
		return fn(key, value)
	})
}

func (rq *RedisQueue) Len() (int, error) {
	count := 0

	err := rq.RangeUnclaimedMessages(func(messageID, key string, value any) bool {
		count++
		return true
	})

	return count, err
}

func (rq *RedisQueue) GetUnclaimedMessagesByKey(key string) ([]*Message, error) {
	var messages []*Message

	messageIDs, err := rq.client.LRange(rq.ctx, rq.getQueueKey(key), 0, -1).Result()
	if err != nil {
		return messages, err
	}

	for _, messageID := range messageIDs {
		// Verifica se não está reivindicada
		claimed, err := rq.client.Exists(rq.ctx, rq.getClaimKey(messageID)).Result()
		if err != nil || claimed > 0 {
			continue
		}

		// Verifica se não foi processada
		if rq.IsProcessed(messageID) {
			continue
		}

		messageData, err := rq.client.Get(rq.ctx, rq.getMessageKey(messageID)).Result()
		if err != nil {
			continue
		}

		var message Message
		if err := json.Unmarshal([]byte(messageData), &message); err != nil {
			continue
		}

		messages = append(messages, &message)
	}

	return messages, nil
}

func (rq *RedisQueue) Close() error {
	return rq.client.Close()
}

func (rq *RedisQueue) Health() error {
	return rq.client.Ping(rq.ctx).Err()
}

// CleanupExpiredClaims limpa claims expirados (deve ser chamado periodicamente)
func (rq *RedisQueue) CleanupExpiredClaims() error {
	claimKeys, err := rq.client.Keys(rq.ctx, rq.prefix+":claims:*").Result()
	if err != nil {
		return err
	}

	now := time.Now().Unix()
	pipe := rq.client.Pipeline()

	for _, claimKey := range claimKeys {
		claimTimeStr, err := rq.client.Get(rq.ctx, claimKey).Result()
		if err != nil {
			continue
		}

		claimTime, err := strconv.ParseInt(claimTimeStr, 10, 64)
		if err != nil {
			continue
		}

		// Se o claim expirou, remove
		if now-claimTime > int64(rq.lockTTL.Seconds()) {
			pipe.Del(rq.ctx, claimKey)
		}
	}

	_, err = pipe.Exec(rq.ctx)
	return err
}
