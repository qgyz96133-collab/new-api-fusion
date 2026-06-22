package relay

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"sort"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relay/common"
)

// SmartRouter implements intelligent routing based on 9router's architecture
type SmartRouter struct {
	mu              sync.RWMutex
	fallbackChains  map[string][]*ProviderChain
	healthChecker   *HealthChecker
	loadBalancer    *LoadBalancer
	routingStrategy RoutingStrategy
}

// ProviderChain represents a chain of providers with fallback
type ProviderChain struct {
	Primary    *model.Channel
	Fallbacks  []*model.Channel
	Weight     int
	Priority   int
	IsHealthy  bool
	LastCheck  time.Time
	Latency    time.Duration
	ErrorCount int
}

// RoutingStrategy defines how to select providers
type RoutingStrategy int

const (
	RoutingStrategyRoundRobin RoutingStrategy = iota
	RoutingStrategyLatencyBased
	RoutingStrategyWeighted
	RoutingStrategyTokenBased
)

// HealthChecker monitors provider health
type HealthChecker struct {
	mu            sync.RWMutex
	healthStatus  map[int]*ProviderHealth
	checkInterval time.Duration
	timeout       time.Duration
}

// ProviderHealth tracks health metrics for a provider
type ProviderHealth struct {
	ChannelID       int
	IsHealthy       bool
	LastCheck       time.Time
	SuccessCount    int
	FailureCount    int
	AverageLatency  time.Duration
	LastLatencies   []time.Duration
	ConsecutiveFail int
}

// LoadBalancer implements various load balancing strategies
type LoadBalancer struct {
	mu           sync.Mutex
	roundRobin   map[string]int
	weights      map[string]int
	tokenCounts  map[string]int64
}

// NewSmartRouter creates a new smart router
func NewSmartRouter() *SmartRouter {
	return &SmartRouter{
		fallbackChains:  make(map[string][]*ProviderChain),
		healthChecker:   NewHealthChecker(),
		loadBalancer:    NewLoadBalancer(),
		routingStrategy: RoutingStrategyLatencyBased,
	}
}

// NewHealthChecker creates a new health checker
func NewHealthChecker() *HealthChecker {
	return &HealthChecker{
		healthStatus:  make(map[int]*ProviderHealth),
		checkInterval: 30 * time.Second,
		timeout:       5 * time.Second,
	}
}

// NewLoadBalancer creates a new load balancer
func NewLoadBalancer() *LoadBalancer {
	return &LoadBalancer{
		roundRobin:  make(map[string]int),
		weights:     make(map[string]int),
		tokenCounts: make(map[string]int64),
	}
}

// SelectProvider selects the best provider for a request
func (r *SmartRouter) SelectProvider(ctx context.Context, model string, tokenCount int) (*model.Channel, error) {
	r.mu.RLock()
	chains, exists := r.fallbackChains[model]
	r.mu.RUnlock()

	if !exists || len(chains) == 0 {
		return nil, fmt.Errorf("no providers available for model: %s", model)
	}

	// Filter healthy providers
	var healthyChains []*ProviderChain
	for _, chain := range chains {
		if chain.IsHealthy {
			healthyChains = append(healthyChains, chain)
		}
	}

	if len(healthyChains) == 0 {
		// All providers are unhealthy, try primary anyway
		healthyChains = chains
	}

	// Apply routing strategy
	var selected *ProviderChain
	switch r.routingStrategy {
	case RoutingStrategyRoundRobin:
		selected = r.selectRoundRobin(healthyChains, model)
	case RoutingStrategyLatencyBased:
		selected = r.selectLowestLatency(healthyChains)
	case RoutingStrategyWeighted:
		selected = r.selectWeighted(healthyChains)
	case RoutingStrategyTokenBased:
		selected = r.selectTokenBased(healthyChains, tokenCount)
	default:
		selected = healthyChains[0]
	}

	if selected == nil {
		return nil, fmt.Errorf("failed to select provider")
	}

	// Try primary first, then fallbacks
	channel := selected.Primary
	if !r.healthChecker.IsHealthy(channel.Id) {
		// Try fallbacks
		for _, fallback := range selected.Fallbacks {
			if r.healthChecker.IsHealthy(fallback.Id) {
				channel = fallback
				break
			}
		}
	}

	return channel, nil
}

// selectRoundRobin selects using round-robin
func (r *SmartRouter) selectRoundRobin(chains []*ProviderChain, model string) *ProviderChain {
	if len(chains) == 0 {
		return nil
	}

	idx := r.loadBalancer.NextRoundRobin(model, len(chains))
	return chains[idx]
}

// selectLowestLatency selects the provider with lowest latency
func (r *SmartRouter) selectLowestLatency(chains []*ProviderChain) *ProviderChain {
	if len(chains) == 0 {
		return nil
	}

	var best *ProviderChain
	var bestLatency time.Duration = time.Duration(1<<63 - 1)

	for _, chain := range chains {
		if chain.Latency > 0 && chain.Latency < bestLatency {
			bestLatency = chain.Latency
			best = chain
		}
	}

	// If no latency data, return first
	if best == nil {
		best = chains[0]
	}

	return best
}

// selectWeighted selects based on weights
func (r *SmartRouter) selectWeighted(chains []*ProviderChain) *ProviderChain {
	if len(chains) == 0 {
		return nil
	}

	totalWeight := 0
	for _, chain := range chains {
		totalWeight += chain.Weight
	}

	if totalWeight == 0 {
		return chains[0]
	}

	randWeight := rand.Intn(totalWeight)
	cumulative := 0
	for _, chain := range chains {
		cumulative += chain.Weight
		if randWeight < cumulative {
			return chain
		}
	}

	return chains[len(chains)-1]
}

// selectTokenBased selects based on token usage
func (r *SmartRouter) selectTokenBased(chains []*ProviderChain, tokenCount int) *ProviderChain {
	if len(chains) == 0 {
		return nil
	}

	// Select provider with lowest token count
	var best *ProviderChain
	var bestTokens int64 = 1<<63 - 1

	for _, chain := range chains {
		tokens := r.loadBalancer.GetTokenCount(chain.Primary.Id)
		if tokens < bestTokens {
			bestTokens = tokens
			best = chain
		}
	}

	if best == nil {
		best = chains[0]
	}

	// Update token count
	r.loadBalancer.AddTokens(best.Primary.Id, int64(tokenCount))

	return best
}

// UpdateHealth updates the health status of a provider
func (r *SmartRouter) UpdateHealth(channelID int, success bool, latency time.Duration) {
	r.healthChecker.UpdateHealth(channelID, success, latency)

	r.mu.Lock()
	defer r.mu.Unlock()

	// Update fallback chains
	for _, chains := range r.fallbackChains {
		for _, chain := range chains {
			if chain.Primary.Id == channelID {
				chain.IsHealthy = r.healthChecker.IsHealthy(channelID)
				chain.LastCheck = time.Now()
				chain.Latency = latency
				if !success {
					chain.ErrorCount++
				} else {
					chain.ErrorCount = 0
				}
			}
		}
	}
}

// RegisterProviderChain registers a provider chain for a model
func (r *SmartRouter) RegisterProviderChain(model string, chain *ProviderChain) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.fallbackChains[model]; !exists {
		r.fallbackChains[model] = make([]*ProviderChain, 0)
	}

	r.fallbackChains[model] = append(r.fallbackChains[model], chain)

	// Sort by priority
	sort.Slice(r.fallbackChains[model], func(i, j int) bool {
		return r.fallbackChains[model][i].Priority < r.fallbackChains[model][j].Priority
	})
}

// IsHealthy checks if a provider is healthy
func (h *HealthChecker) IsHealthy(channelID int) bool {
	h.mu.RLock()
	defer h.mu.RUnlock()

	health, exists := h.healthStatus[channelID]
	if !exists {
		return true // Assume healthy if no data
	}

	return health.IsHealthy
}

// UpdateHealth updates health status for a provider
func (h *HealthChecker) UpdateHealth(channelID int, success bool, latency time.Duration) {
	h.mu.Lock()
	defer h.mu.Unlock()

	health, exists := h.healthStatus[channelID]
	if !exists {
		health = &ProviderHealth{
			ChannelID:     channelID,
			IsHealthy:     true,
			LastLatencies: make([]time.Duration, 0, 10),
		}
		h.healthStatus[channelID] = health
	}

	health.LastCheck = time.Now()

	if success {
		health.SuccessCount++
		health.ConsecutiveFail = 0

		// Update latency (rolling average of last 10)
		health.LastLatencies = append(health.LastLatencies, latency)
		if len(health.LastLatencies) > 10 {
			health.LastLatencies = health.LastLatencies[1:]
		}

		totalLatency := time.Duration(0)
		for _, l := range health.LastLatencies {
			totalLatency += l
		}
		health.AverageLatency = totalLatency / time.Duration(len(health.LastLatencies))

		// Mark as healthy after 3 consecutive successes
		if health.SuccessCount >= 3 {
			health.IsHealthy = true
		}
	} else {
		health.FailureCount++
		health.ConsecutiveFail++

		// Mark as unhealthy after 3 consecutive failures
		if health.ConsecutiveFail >= 3 {
			health.IsHealthy = false
		}
	}
}

// GetHealth returns the health status for a channel
// 获取通道的健康状态
func (h *HealthChecker) GetHealth(channelID int) *ProviderHealth {
	h.mu.RLock()
	defer h.mu.RUnlock()

	return h.healthStatus[channelID]
}

// NextRoundRobin returns the next index for round-robin
func (lb *LoadBalancer) NextRoundRobin(key string, count int) int {
	lb.mu.Lock()
	defer lb.mu.Unlock()

	current := lb.roundRobin[key]
	next := (current + 1) % count
	lb.roundRobin[key] = next
	return next
}

// GetTokenCount gets the token count for a channel
func (lb *LoadBalancer) GetTokenCount(channelID int) int64 {
	lb.mu.Lock()
	defer lb.mu.Unlock()

	key := fmt.Sprintf("channel_%d", channelID)
	return lb.tokenCounts[key]
}

// AddTokens adds tokens to a channel's count
func (lb *LoadBalancer) AddTokens(channelID int, tokens int64) {
	lb.mu.Lock()
	defer lb.mu.Unlock()

	key := fmt.Sprintf("channel_%d", channelID)
	lb.tokenCounts[key] += tokens
}

// SelectHealthyChannel selects the healthiest channel from a list of candidates
// 从候选通道列表中选择最健康的通道
func (r *SmartRouter) SelectHealthyChannel(channels []*model.Channel) *model.Channel {
	if len(channels) == 0 {
		return nil
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	// Score each channel based on health and performance
	type scoredChannel struct {
		channel *model.Channel
		score   float64
	}

	scored := make([]scoredChannel, 0, len(channels))
	for _, ch := range channels {
		if ch.Status != 1 {
			continue // Skip disabled channels
		}

		health := r.healthChecker.GetHealth(ch.Id)
		if health == nil {
			// No health data, assume healthy with neutral score
			scored = append(scored, scoredChannel{channel: ch, score: 1.0})
			continue
		}

		// Calculate health score based on:
		// - Success rate (weight: 0.4)
		// - Average latency (weight: 0.3)
		// - Consecutive failures penalty (weight: 0.3)
		score := 0.0

		// Success rate component
		totalRequests := health.SuccessCount + health.FailureCount
		if totalRequests > 0 {
			successRate := float64(health.SuccessCount) / float64(totalRequests)
			score += successRate * 0.4
		} else {
			score += 0.5 * 0.4 // Neutral score for new channels
		}

		// Latency component (lower is better, normalized)
		if health.AverageLatency > 0 {
			// Assume 5 seconds as "slow", anything faster gets higher score
			latencyScore := 1.0 - (float64(health.AverageLatency.Milliseconds()) / 5000.0)
			if latencyScore < 0 {
				latencyScore = 0
			}
			score += latencyScore * 0.3
		} else {
			score += 0.5 * 0.3
		}

		// Consecutive failures penalty
		if health.ConsecutiveFail == 0 {
			score += 1.0 * 0.3
		} else if health.ConsecutiveFail <= 2 {
			score += 0.5 * 0.3
		} else {
			score += 0.0 * 0.3 // Heavy penalty for unhealthy channels
		}

		// Skip unhealthy channels unless they're the only option
		if !health.IsHealthy {
			score *= 0.1 // 90% penalty for unhealthy channels
		}

		scored = append(scored, scoredChannel{channel: ch, score: score})
	}

	if len(scored) == 0 {
		return nil
	}

	// Sort by score descending
	sort.Slice(scored, func(i, j int) bool {
		return scored[i].score > scored[j].score
	})

	// Return the best channel
	return scored[0].channel
}

// GetChannelHealthScore returns the health score for a channel (0.0 to 1.0)
// 获取通道的健康分数（0.0 到 1.0）
func (r *SmartRouter) GetChannelHealthScore(channelID int) float64 {
	r.mu.RLock()
	defer r.mu.RUnlock()

	health := r.healthChecker.GetHealth(channelID)
	if health == nil {
		return 1.0 // Assume healthy if no data
	}

	score := 0.0

	// Success rate
	totalRequests := health.SuccessCount + health.FailureCount
	if totalRequests > 0 {
		successRate := float64(health.SuccessCount) / float64(totalRequests)
		score += successRate * 0.4
	} else {
		score += 0.5 * 0.4
	}

	// Latency
	if health.AverageLatency > 0 {
		latencyScore := 1.0 - (float64(health.AverageLatency.Milliseconds()) / 5000.0)
		if latencyScore < 0 {
			latencyScore = 0
		}
		score += latencyScore * 0.3
	} else {
		score += 0.5 * 0.3
	}

	// Consecutive failures
	if health.ConsecutiveFail == 0 {
		score += 1.0 * 0.3
	} else if health.ConsecutiveFail <= 2 {
		score += 0.5 * 0.3
	}

	if !health.IsHealthy {
		score *= 0.1
	}

	return score
}

// BuildFallbackChainsFromChannels builds fallback chains from channel data
func BuildFallbackChainsFromChannels(channels []*model.Channel) map[string][]*ProviderChain {
	chains := make(map[string][]*ProviderChain)

	// Group channels by model
	modelChannels := make(map[string][]*model.Channel)
	for _, channel := range channels {
		if channel.Status != 1 {
			continue
		}

		// Parse models from channel
		var models []string
		if channel.ModelMapping != nil {
			if err := json.Unmarshal([]byte(*channel.ModelMapping), &models); err != nil {
				// Try single model
				models = []string{*channel.ModelMapping}
			}
		}

		for _, m := range models {
			modelChannels[m] = append(modelChannels[m], channel)
		}
	}

	// Build chains for each model
	for m, chans := range modelChannels {
		if len(chans) == 0 {
			continue
		}

		// Sort by priority
		sort.Slice(chans, func(i, j int) bool {
			priorityI := int64(0)
			priorityJ := int64(0)
			if chans[i].Priority != nil {
				priorityI = *chans[i].Priority
			}
			if chans[j].Priority != nil {
				priorityJ = *chans[j].Priority
			}
			return priorityI < priorityJ
		})

		// Create chain
		weight := uint(0)
		priority := int64(0)
		if chans[0].Weight != nil {
			weight = *chans[0].Weight
		}
		if chans[0].Priority != nil {
			priority = *chans[0].Priority
		}

		chain := &ProviderChain{
			Primary:   chans[0],
			Fallbacks: chans[1:],
			Weight:    int(weight),
			Priority:  int(priority),
			IsHealthy: true,
		}

		chains[m] = []*ProviderChain{chain}
	}

	return chains
}

// SmartRouterMiddleware integrates smart routing into the relay
func SmartRouterMiddleware(router *SmartRouter) func(*common.RelayInfo) (*model.Channel, error) {
	return func(info *common.RelayInfo) (*model.Channel, error) {
		ctx := context.Background()

		// Estimate token count
		tokenCount := estimateTokenCount(info)

		// Select provider
		channel, err := router.SelectProvider(ctx, info.UpstreamModelName, tokenCount)
		if err != nil {
			return nil, err
		}

		return channel, nil
	}
}

// estimateTokenCount estimates token count from request
func estimateTokenCount(info *common.RelayInfo) int {
	// TODO: Implement based on actual message structure
	// For now, return a default estimate
	return 1000
}
