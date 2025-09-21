package load

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

// RedisBalancer implementação Redis da interface Balancer
type RedisBalancer struct {
	client  *redis.Client
	config  *BalancerConfig
	ctx     context.Context
	prefix  string
	counter string
}

// NewRedisBalancer cria uma nova instância do balancer Redis
func NewRedisBalancer(config *BalancerConfig) (*RedisBalancer, error) {
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

	if config.HeartbeatTTL == 0 {
		config.HeartbeatTTL = 30 * time.Second
	}
	if config.CleanupTicker == 0 {
		config.CleanupTicker = 60 * time.Second
	}
	if config.Strategy == "" {
		config.Strategy = StrategyRoundRobin
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

	prefix := "balancer"
	if config.Prefix != "" {
		prefix = config.Prefix
	}
	if config.AppName != "" {
		prefix = fmt.Sprintf("%s:%s", prefix, config.AppName)
	}

	rb := &RedisBalancer{
		client:  client,
		config:  config,
		ctx:     ctx,
		prefix:  prefix,
		counter: fmt.Sprintf("%s:counter", prefix),
	}

	// Inicia cleanup automático
	go rb.startCleanupTicker()

	return rb, nil
}

func (rb *RedisBalancer) getSubscriberKey(subID uuid.UUID) string {
	return fmt.Sprintf("%s:subscribers:%s", rb.prefix, subID.String())
}

func (rb *RedisBalancer) getActiveSetKey() string {
	return fmt.Sprintf("%s:active", rb.prefix)
}

func (rb *RedisBalancer) getClaimKey(messageID string) string {
	return fmt.Sprintf("%s:claims:%s", rb.prefix, messageID)
}

func (rb *RedisBalancer) getStatsKey() string {
	return fmt.Sprintf("%s:stats", rb.prefix)
}

func (rb *RedisBalancer) Subscribe(subID uuid.UUID) error {
	now := time.Now().Unix()

	subscriber := &SubscriberInfo{
		ID:           subID,
		Active:       true,
		LastSeen:     now,
		Weight:       1,
		ProcessCount: 0,
		JoinedAt:     now,
	}

	data, err := json.Marshal(subscriber)
	if err != nil {
		return fmt.Errorf("failed to marshal subscriber: %w", err)
	}

	pipe := rb.client.Pipeline()

	// Armazena informações do subscriber
	pipe.Set(rb.ctx, rb.getSubscriberKey(subID), data, rb.config.HeartbeatTTL*2)

	// Adiciona ao conjunto de ativos
	pipe.SAdd(rb.ctx, rb.getActiveSetKey(), subID.String())
	pipe.Expire(rb.ctx, rb.getActiveSetKey(), rb.config.HeartbeatTTL*2)

	// Atualiza estatísticas
	pipe.HIncrBy(rb.ctx, rb.getStatsKey(), "total_subscribers", 1)
	pipe.HIncrBy(rb.ctx, rb.getStatsKey(), "active_subscribers", 1)

	_, err = pipe.Exec(rb.ctx)
	return err
}

func (rb *RedisBalancer) Unsubscribe(subID uuid.UUID) error {
	pipe := rb.client.Pipeline()

	// Remove informações do subscriber
	pipe.Del(rb.ctx, rb.getSubscriberKey(subID))

	// Remove do conjunto de ativos
	pipe.SRem(rb.ctx, rb.getActiveSetKey(), subID.String())

	// Atualiza estatísticas
	pipe.HIncrBy(rb.ctx, rb.getStatsKey(), "active_subscribers", -1)

	_, err := pipe.Exec(rb.ctx)
	return err
}

func (rb *RedisBalancer) GetTotalSubscribers() (int, error) {
	count, err := rb.client.SCard(rb.ctx, rb.getActiveSetKey()).Result()
	if err != nil {
		return 0, err
	}
	return int(count), nil
}

func (rb *RedisBalancer) ShouldProcess(subID uuid.UUID, messageKey string) (bool, error) {
	activeSubscribers, err := rb.GetActiveSubscribers()
	if err != nil {
		return false, err
	}

	if len(activeSubscribers) == 0 {
		return false, nil
	}

	switch rb.config.Strategy {
	case StrategyRoundRobin:
		return rb.roundRobinStrategy(subID, activeSubscribers)
	case StrategyConsistentHash:
		return rb.consistentHashStrategy(subID, messageKey, activeSubscribers)
	case StrategyWeighted:
		return rb.weightedStrategy(subID, activeSubscribers)
	case StrategyLeastConn:
		return rb.leastConnStrategy(subID, activeSubscribers)
	default:
		return rb.roundRobinStrategy(subID, activeSubscribers)
	}
}

func (rb *RedisBalancer) ClaimMessage(subID uuid.UUID, messageID string) (bool, error) {
	// Verifica se o subscriber está ativo
	isActive, err := rb.client.SIsMember(rb.ctx, rb.getActiveSetKey(), subID.String()).Result()
	if err != nil || !isActive {
		return false, err
	}

	// Tenta criar um claim atômico
	claimKey := rb.getClaimKey(messageID)
	result, err := rb.client.SetNX(rb.ctx, claimKey, subID.String(), rb.config.HeartbeatTTL).Result()
	if err != nil || !result {
		return false, err
	}

	// Atualiza informações do subscriber
	go rb.updateSubscriberStats(subID)

	return true, nil
}

func (rb *RedisBalancer) GetActiveSubscribers() ([]uuid.UUID, error) {
	members, err := rb.client.SMembers(rb.ctx, rb.getActiveSetKey()).Result()
	if err != nil {
		return nil, err
	}

	subscribers := make([]uuid.UUID, 0, len(members))
	for _, member := range members {
		if id, err := uuid.Parse(member); err == nil {
			subscribers = append(subscribers, id)
		}
	}

	return subscribers, nil
}

func (rb *RedisBalancer) UpdateHeartbeat(subID uuid.UUID) error {
	// Atualiza TTL do subscriber
	key := rb.getSubscriberKey(subID)
	exists, err := rb.client.Exists(rb.ctx, key).Result()
	if err != nil {
		return err
	}

	if exists > 0 {
		// Atualiza último seen
		now := time.Now().Unix()
		luaScript := `
			local key = KEYS[1]
			local data = redis.call('GET', key)
			if data then
				local subscriber = cjson.decode(data)
				subscriber.last_seen = tonumber(ARGV[1])
				redis.call('SET', key, cjson.encode(subscriber), 'EX', tonumber(ARGV[2]))
				return 1
			end
			return 0
		`

		_, err = rb.client.Eval(rb.ctx, luaScript, []string{key}, now, int64(rb.config.HeartbeatTTL.Seconds())).Result()
		if err != nil {
			return err
		}

		// Atualiza TTL do conjunto ativo
		rb.client.Expire(rb.ctx, rb.getActiveSetKey(), rb.config.HeartbeatTTL*2)
	}

	return nil
}

func (rb *RedisBalancer) CleanupInactiveSubscribers(timeout time.Duration) error {
	cutoff := time.Now().Add(-timeout).Unix()

	// Script Lua para limpeza atômica
	luaScript := `
		local activeKey = KEYS[1]
		local subscribersPrefix = ARGV[1]
		local cutoff = tonumber(ARGV[2])
		local removed = 0

		local members = redis.call('SMEMBERS', activeKey)
		for i=1,#members do
			local subKey = subscribersPrefix .. ':' .. members[i]
			local data = redis.call('GET', subKey)
			if data then
				local subscriber = cjson.decode(data)
				if subscriber.last_seen < cutoff then
					redis.call('SREM', activeKey, members[i])
					redis.call('DEL', subKey)
					removed = removed + 1
				end
			else
				redis.call('SREM', activeKey, members[i])
				removed = removed + 1
			end
		end

		-- Atualiza estatísticas
		if removed > 0 then
			redis.call('HINCRBY', KEYS[2], 'active_subscribers', -removed)
		end

		return removed
	`

	_, err := rb.client.Eval(rb.ctx, luaScript,
		[]string{rb.getActiveSetKey(), rb.getStatsKey()},
		fmt.Sprintf("%s:subscribers", rb.prefix),
		cutoff,
	).Result()

	return err
}

func (rb *RedisBalancer) GetSubscriberInfo(subID uuid.UUID) (*SubscriberInfo, error) {
	data, err := rb.client.Get(rb.ctx, rb.getSubscriberKey(subID)).Result()
	if err != nil {
		if err == redis.Nil {
			return nil, nil
		}
		return nil, err
	}

	var subscriber SubscriberInfo
	if err := json.Unmarshal([]byte(data), &subscriber); err != nil {
		return nil, err
	}

	return &subscriber, nil
}

func (rb *RedisBalancer) SetSubscriberWeight(subID uuid.UUID, weight int) error {
	key := rb.getSubscriberKey(subID)

	luaScript := `
		local key = KEYS[1]
		local weight = tonumber(ARGV[1])
		local ttl = tonumber(ARGV[2])

		local data = redis.call('GET', key)
		if data then
			local subscriber = cjson.decode(data)
			subscriber.weight = weight
			redis.call('SET', key, cjson.encode(subscriber), 'EX', ttl)
			return 1
		end
		return 0
	`

	_, err := rb.client.Eval(rb.ctx, luaScript, []string{key}, weight, int64(rb.config.HeartbeatTTL.Seconds())).Result()
	return err
}

func (rb *RedisBalancer) Close() error {
	return rb.client.Close()
}

func (rb *RedisBalancer) Health() error {
	return rb.client.Ping(rb.ctx).Err()
}

// Estratégias de balanceamento

func (rb *RedisBalancer) roundRobinStrategy(subID uuid.UUID, activeSubscribers []uuid.UUID) (bool, error) {
	counter, err := rb.client.Incr(rb.ctx, rb.counter).Result()
	if err != nil {
		return false, err
	}

	index := (counter - 1) % int64(len(activeSubscribers))
	selectedID := activeSubscribers[index]
	return selectedID == subID, nil
}

func (rb *RedisBalancer) consistentHashStrategy(subID uuid.UUID, messageKey string, activeSubscribers []uuid.UUID) (bool, error) {
	hash := rb.simpleHash(messageKey)
	index := hash % len(activeSubscribers)
	selectedID := activeSubscribers[index]
	return selectedID == subID, nil
}

func (rb *RedisBalancer) weightedStrategy(subID uuid.UUID, activeSubscribers []uuid.UUID) (bool, error) {
	// Busca pesos dos subscribers
	weights := make(map[uuid.UUID]int)
	totalWeight := 0

	for _, id := range activeSubscribers {
		sub, err := rb.GetSubscriberInfo(id)
		if err != nil || sub == nil {
			weights[id] = 1
		} else {
			weight := sub.Weight
			if weight <= 0 {
				weight = 1
			}
			weights[id] = weight
		}
		totalWeight += weights[id]
	}

	if totalWeight == 0 {
		return rb.roundRobinStrategy(subID, activeSubscribers)
	}

	counter, err := rb.client.Incr(rb.ctx, rb.counter).Result()
	if err != nil {
		return false, err
	}

	target := int(counter-1) % totalWeight
	current := 0

	for _, id := range activeSubscribers {
		current += weights[id]
		if current > target {
			return id == subID, nil
		}
	}

	return false, nil
}

func (rb *RedisBalancer) leastConnStrategy(subID uuid.UUID, activeSubscribers []uuid.UUID) (bool, error) {
	var minCount int64 = -1
	var selectedID uuid.UUID

	for _, id := range activeSubscribers {
		sub, err := rb.GetSubscriberInfo(id)
		if err != nil || sub == nil {
			continue
		}

		if minCount == -1 || sub.ProcessCount < minCount {
			minCount = sub.ProcessCount
			selectedID = id
		}
	}

	return selectedID == subID, nil
}

// Funções auxiliares

func (rb *RedisBalancer) simpleHash(s string) int {
	hash := 0
	for _, c := range s {
		hash = hash*31 + int(c)
	}
	if hash < 0 {
		hash = -hash
	}
	return hash
}

func (rb *RedisBalancer) updateSubscriberStats(subID uuid.UUID) {
	key := rb.getSubscriberKey(subID)
	now := time.Now().Unix()

	luaScript := `
		local key = KEYS[1]
		local now = tonumber(ARGV[1])
		local ttl = tonumber(ARGV[2])

		local data = redis.call('GET', key)
		if data then
			local subscriber = cjson.decode(data)
			subscriber.process_count = subscriber.process_count + 1
			subscriber.last_seen = now
			redis.call('SET', key, cjson.encode(subscriber), 'EX', ttl)

			-- Atualiza estatísticas globais
			redis.call('HINCRBY', KEYS[2], 'messages_processed', 1)
		end
	`

	rb.client.Eval(rb.ctx, luaScript,
		[]string{key, rb.getStatsKey()},
		now,
		int64(rb.config.HeartbeatTTL.Seconds()),
	)
}

func (rb *RedisBalancer) startCleanupTicker() {
	ticker := time.NewTicker(rb.config.CleanupTicker)
	defer ticker.Stop()

	for range ticker.C {
		rb.CleanupInactiveSubscribers(rb.config.HeartbeatTTL)
	}
}

// GetStats retorna estatísticas do balancer
func (rb *RedisBalancer) GetStats() (*BalancerStats, error) {
	stats, err := rb.client.HMGet(rb.ctx, rb.getStatsKey(),
		"total_subscribers",
		"active_subscribers",
		"messages_processed",
	).Result()
	if err != nil {
		return nil, err
	}

	result := &BalancerStats{}

	if stats[0] != nil {
		if val, err := strconv.Atoi(stats[0].(string)); err == nil {
			result.TotalSubscribers = val
		}
	}

	if stats[1] != nil {
		if val, err := strconv.Atoi(stats[1].(string)); err == nil {
			result.ActiveSubscribers = val
		}
	}

	if stats[2] != nil {
		if val, err := strconv.ParseInt(stats[2].(string), 10, 64); err == nil {
			result.MessagesProcessed = val
		}
	}

	return result, nil
}
