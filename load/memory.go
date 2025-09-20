package load

import (
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
)

// MemoryBalancer implementação em memória da interface Balancer
type MemoryBalancer struct {
	appName     string
	subscribers map[uuid.UUID]*SubscriberInfo
	mutex       sync.RWMutex
	counter     int64
	config      *BalancerConfig
	stats       *BalancerStats
	statsMutex  sync.RWMutex
}

// NewMemoryBalancer cria uma nova instância do balancer em memória
func NewMemoryBalancer(config *BalancerConfig) *MemoryBalancer {
	if config.HeartbeatTTL == 0 {
		config.HeartbeatTTL = 30 * time.Second
	}
	if config.CleanupTicker == 0 {
		config.CleanupTicker = 60 * time.Second
	}
	if config.Strategy == "" {
		config.Strategy = StrategyRoundRobin
	}

	mb := &MemoryBalancer{
		appName:     config.AppName,
		subscribers: make(map[uuid.UUID]*SubscriberInfo),
		counter:     0,
		config:      config,
		stats: &BalancerStats{
			LastCleanup: time.Now(),
		},
	}

	// Inicia cleanup automático
	go mb.startCleanupTicker()

	return mb
}

func (mb *MemoryBalancer) Subscribe(subID uuid.UUID) error {
	mb.mutex.Lock()
	defer mb.mutex.Unlock()

	now := time.Now().Unix()
	mb.subscribers[subID] = &SubscriberInfo{
		ID:           subID,
		Active:       true,
		LastSeen:     now,
		Weight:       1, // peso padrão
		ProcessCount: 0,
		JoinedAt:     now,
	}

	mb.updateStats()
	return nil
}

func (mb *MemoryBalancer) Unsubscribe(subID uuid.UUID) error {
	mb.mutex.Lock()
	defer mb.mutex.Unlock()

	if sub, exists := mb.subscribers[subID]; exists {
		sub.Active = false
		delete(mb.subscribers, subID)
	}

	mb.updateStats()
	return nil
}

func (mb *MemoryBalancer) GetTotalSubscribers() (int, error) {
	mb.mutex.RLock()
	defer mb.mutex.RUnlock()

	count := 0
	for _, sub := range mb.subscribers {
		if sub.Active {
			count++
		}
	}
	return count, nil
}

func (mb *MemoryBalancer) ShouldProcess(subID uuid.UUID, messageKey string) (bool, error) {
	mb.mutex.RLock()
	defer mb.mutex.RUnlock()

	activeSubscribers := make([]uuid.UUID, 0)
	for id, sub := range mb.subscribers {
		if sub.Active {
			activeSubscribers = append(activeSubscribers, id)
		}
	}

	if len(activeSubscribers) == 0 {
		return false, nil
	}

	switch mb.config.Strategy {
	case StrategyRoundRobin:
		return mb.roundRobinStrategy(subID, activeSubscribers), nil
	case StrategyConsistentHash:
		return mb.consistentHashStrategy(subID, messageKey, activeSubscribers), nil
	case StrategyWeighted:
		return mb.weightedStrategy(subID, activeSubscribers), nil
	case StrategyLeastConn:
		return mb.leastConnStrategy(subID, activeSubscribers), nil
	default:
		return mb.roundRobinStrategy(subID, activeSubscribers), nil
	}
}

func (mb *MemoryBalancer) ClaimMessage(subID uuid.UUID, messageID string) (bool, error) {
	mb.mutex.RLock()
	defer mb.mutex.RUnlock()

	// Verifica se o subscriber está ativo
	sub, exists := mb.subscribers[subID]
	if !exists || !sub.Active {
		return false, nil
	}

	activeCount := 0
	subscriberIndex := -1
	currentIndex := 0

	for id, subscriber := range mb.subscribers {
		if subscriber.Active {
			if id == subID {
				subscriberIndex = currentIndex
			}
			activeCount++
			currentIndex++
		}
	}

	if activeCount == 0 || subscriberIndex == -1 {
		return false, nil
	}

	// Usa hash simples do messageID para distribuição consistente
	hash := mb.simpleHash(messageID)
	targetIndex := hash % activeCount

	shouldProcess := subscriberIndex == targetIndex
	if shouldProcess {
		// Atualiza contadores
		sub.ProcessCount++
		sub.LastSeen = time.Now().Unix()

		mb.statsMutex.Lock()
		mb.stats.MessagesProcessed++
		mb.statsMutex.Unlock()
	}

	return shouldProcess, nil
}

func (mb *MemoryBalancer) GetActiveSubscribers() ([]uuid.UUID, error) {
	mb.mutex.RLock()
	defer mb.mutex.RUnlock()

	active := make([]uuid.UUID, 0)
	for id, sub := range mb.subscribers {
		if sub.Active {
			active = append(active, id)
		}
	}
	return active, nil
}

func (mb *MemoryBalancer) UpdateHeartbeat(subID uuid.UUID) error {
	mb.mutex.Lock()
	defer mb.mutex.Unlock()

	if sub, exists := mb.subscribers[subID]; exists && sub.Active {
		sub.LastSeen = time.Now().Unix()
	}
	return nil
}

func (mb *MemoryBalancer) CleanupInactiveSubscribers(timeout time.Duration) error {
	mb.mutex.Lock()
	defer mb.mutex.Unlock()

	now := time.Now().Unix()
	cutoff := now - int64(timeout.Seconds())

	for id, sub := range mb.subscribers {
		if sub.LastSeen < cutoff {
			sub.Active = false
			delete(mb.subscribers, id)
		}
	}

	mb.updateStats()
	mb.statsMutex.Lock()
	mb.stats.LastCleanup = time.Now()
	mb.statsMutex.Unlock()

	return nil
}

func (mb *MemoryBalancer) GetSubscriberInfo(subID uuid.UUID) (*SubscriberInfo, error) {
	mb.mutex.RLock()
	defer mb.mutex.RUnlock()

	if sub, exists := mb.subscribers[subID]; exists {
		// Retorna uma cópia para evitar modificações acidentais
		return &SubscriberInfo{
			ID:           sub.ID,
			Active:       sub.Active,
			LastSeen:     sub.LastSeen,
			Weight:       sub.Weight,
			ProcessCount: sub.ProcessCount,
			JoinedAt:     sub.JoinedAt,
		}, nil
	}
	return nil, nil
}

func (mb *MemoryBalancer) SetSubscriberWeight(subID uuid.UUID, weight int) error {
	mb.mutex.Lock()
	defer mb.mutex.Unlock()

	if sub, exists := mb.subscribers[subID]; exists && sub.Active {
		sub.Weight = weight
	}
	return nil
}

func (mb *MemoryBalancer) Close() error {
	// Não há recursos para fechar na implementação em memória
	return nil
}

func (mb *MemoryBalancer) Health() error {
	// Implementação em memória sempre está saudável
	return nil
}

// Estratégias de balanceamento

func (mb *MemoryBalancer) roundRobinStrategy(subID uuid.UUID, activeSubscribers []uuid.UUID) bool {
	index := atomic.AddInt64(&mb.counter, 1) % int64(len(activeSubscribers))
	selectedID := activeSubscribers[index]
	return selectedID == subID
}

func (mb *MemoryBalancer) consistentHashStrategy(subID uuid.UUID, messageKey string, activeSubscribers []uuid.UUID) bool {
	hash := mb.simpleHash(messageKey)
	index := hash % len(activeSubscribers)
	selectedID := activeSubscribers[index]
	return selectedID == subID
}

func (mb *MemoryBalancer) weightedStrategy(subID uuid.UUID, activeSubscribers []uuid.UUID) bool {
	// Calcula peso total
	totalWeight := 0
	weights := make(map[uuid.UUID]int)

	for _, id := range activeSubscribers {
		if sub, exists := mb.subscribers[id]; exists {
			weight := sub.Weight
			if weight <= 0 {
				weight = 1
			}
			weights[id] = weight
			totalWeight += weight
		}
	}

	if totalWeight == 0 {
		return mb.roundRobinStrategy(subID, activeSubscribers)
	}

	// Escolhe baseado no peso
	target := int(atomic.AddInt64(&mb.counter, 1)) % totalWeight
	current := 0

	for _, id := range activeSubscribers {
		current += weights[id]
		if current > target {
			return id == subID
		}
	}

	return false
}

func (mb *MemoryBalancer) leastConnStrategy(subID uuid.UUID, activeSubscribers []uuid.UUID) bool {
	var minCount int64 = -1
	var selectedID uuid.UUID

	for _, id := range activeSubscribers {
		if sub, exists := mb.subscribers[id]; exists {
			if minCount == -1 || sub.ProcessCount < minCount {
				minCount = sub.ProcessCount
				selectedID = id
			}
		}
	}

	return selectedID == subID
}

// Funções auxiliares

func (mb *MemoryBalancer) simpleHash(s string) int {
	hash := 0
	for _, c := range s {
		hash = hash*31 + int(c)
	}
	if hash < 0 {
		hash = -hash
	}
	return hash
}

func (mb *MemoryBalancer) updateStats() {
	total := 0
	active := 0

	for _, sub := range mb.subscribers {
		total++
		if sub.Active {
			active++
		}
	}

	mb.statsMutex.Lock()
	mb.stats.TotalSubscribers = total
	mb.stats.ActiveSubscribers = active
	mb.statsMutex.Unlock()
}

func (mb *MemoryBalancer) startCleanupTicker() {
	ticker := time.NewTicker(mb.config.CleanupTicker)
	defer ticker.Stop()

	for range ticker.C {
		mb.CleanupInactiveSubscribers(mb.config.HeartbeatTTL)
	}
}

// GetStats retorna estatísticas do balancer
func (mb *MemoryBalancer) GetStats() *BalancerStats {
	mb.statsMutex.RLock()
	defer mb.statsMutex.RUnlock()

	return &BalancerStats{
		TotalSubscribers:  mb.stats.TotalSubscribers,
		ActiveSubscribers: mb.stats.ActiveSubscribers,
		MessagesProcessed: mb.stats.MessagesProcessed,
		LastCleanup:       mb.stats.LastCleanup,
	}
}
