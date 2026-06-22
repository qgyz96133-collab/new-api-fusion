package relay

import (
	"sync"
	"time"

	"github.com/QuantumNous/new-api/model"
)

// GlobalSmartRouter is the global smart router instance
var (
	globalSmartRouter     *SmartRouter
	globalSmartRouterOnce sync.Once
)

// GetGlobalSmartRouter returns the global smart router instance
func GetGlobalSmartRouter() *SmartRouter {
	globalSmartRouterOnce.Do(func() {
		globalSmartRouter = NewSmartRouter()
		// Set default routing strategy
		globalSmartRouter.routingStrategy = RoutingStrategyWeighted
	})
	return globalSmartRouter
}

// InitializeSmartRouterWithChannels initializes the smart router with existing channels
func InitializeSmartRouterWithChannels() error {
	channels, err := model.GetAllChannels(0, 0, true, false)
	if err != nil {
		return err
	}

	router := GetGlobalSmartRouter()
	chains := BuildFallbackChainsFromChannels(channels)

	router.mu.Lock()
	defer router.mu.Unlock()

	for model, chain := range chains {
		router.fallbackChains[model] = chain
	}

	return nil
}

// UpdateChannelHealth updates the health status of a channel in the smart router
func UpdateChannelHealth(channelID int, success bool, latency time.Duration) {
	router := GetGlobalSmartRouter()
	router.UpdateHealth(channelID, success, latency)
}

// RefreshSmartRouter refreshes the smart router with latest channel data
func RefreshSmartRouter() error {
	channels, err := model.GetAllChannels(0, 0, true, false)
	if err != nil {
		return err
	}

	router := GetGlobalSmartRouter()
	chains := BuildFallbackChainsFromChannels(channels)

	router.mu.Lock()
	defer router.mu.Unlock()

	// Clear existing chains
	router.fallbackChains = make(map[string][]*ProviderChain)

	// Add new chains
	for model, chain := range chains {
		router.fallbackChains[model] = chain
	}

	return nil
}
