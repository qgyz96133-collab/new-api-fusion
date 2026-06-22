package service

import (
	"errors"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting"
	"github.com/gin-gonic/gin"
)

// SmartRouterIface abstracts relay.SmartRouter to break the service → relay import cycle.
type SmartRouterIface interface {
	SelectHealthyChannel(channels []*model.Channel) *model.Channel
	GetChannelHealthScore(channelID int) float64
}

// GetSmartRouterFunc is injected by main.go to return the global SmartRouter instance.
// This breaks the service → relay → relay/channel → service circular import.
var GetSmartRouterFunc func() SmartRouterIface

type RetryParam struct {
	Ctx          *gin.Context
	TokenGroup   string
	ModelName    string
	Retry        *int
	resetNextTry bool
}

func (p *RetryParam) GetRetry() int {
	if p.Retry == nil {
		return 0
	}
	return *p.Retry
}

func (p *RetryParam) SetRetry(retry int) {
	p.Retry = &retry
}

func (p *RetryParam) IncreaseRetry() {
	if p.resetNextTry {
		p.resetNextTry = false
		return
	}
	if p.Retry == nil {
		p.Retry = new(int)
	}
	*p.Retry++
}

func (p *RetryParam) ResetRetryNextTry() {
	p.resetNextTry = true
}

// CacheGetRandomSatisfiedChannel tries to get a random channel that satisfies the requirements.
// 尝试获取一个满足要求的随机渠道。
// Now integrated with SmartRouter for health-aware routing.
// 现已集成 SmartRouter 以实现健康感知路由。
func CacheGetRandomSatisfiedChannel(param *RetryParam) (*model.Channel, string, error) {
	// Try SmartRouter first for health-aware selection
	var smartRouter SmartRouterIface
	if GetSmartRouterFunc != nil {
		smartRouter = GetSmartRouterFunc()
	}
	if smartRouter != nil {
		channel, group, err := selectChannelWithSmartRouter(param, smartRouter)
		if channel != nil || err != nil {
			return channel, group, err
		}
		// If SmartRouter returns nil but no error, fall back to original logic
		logger.LogDebug(param.Ctx, "SmartRouter returned no channel, falling back to original logic")
	}

	// Original logic (fallback)
	var channel *model.Channel
	var err error
	selectGroup := param.TokenGroup
	userGroup := common.GetContextKeyString(param.Ctx, constant.ContextKeyUserGroup)

	if param.TokenGroup == "auto" {
		if len(setting.GetAutoGroups()) == 0 {
			return nil, selectGroup, errors.New("auto groups is not enabled")
		}
		autoGroups := GetUserAutoGroup(userGroup)

		// startGroupIndex: the group index to start searching from
		// startGroupIndex: 开始搜索的分组索引
		startGroupIndex := 0
		crossGroupRetry := common.GetContextKeyBool(param.Ctx, constant.ContextKeyTokenCrossGroupRetry)

		if lastGroupIndex, exists := common.GetContextKey(param.Ctx, constant.ContextKeyAutoGroupIndex); exists {
			if idx, ok := lastGroupIndex.(int); ok {
				startGroupIndex = idx
			}
		}

		for i := startGroupIndex; i < len(autoGroups); i++ {
			autoGroup := autoGroups[i]
			// Calculate priorityRetry for current group
			// 计算当前分组的 priorityRetry
			priorityRetry := param.GetRetry()
			// If moved to a new group, reset priorityRetry and update startRetryIndex
			// 如果切换到新分组，重置 priorityRetry 并更新 startRetryIndex
			if i > startGroupIndex {
				priorityRetry = 0
			}
			logger.LogDebug(param.Ctx, "Auto selecting group: %s, priorityRetry: %d", autoGroup, priorityRetry)

			channel, _ = model.GetRandomSatisfiedChannel(autoGroup, param.ModelName, priorityRetry)
			if channel == nil {
				// Current group has no available channel for this model, try next group
				// 当前分组没有该模型的可用渠道，尝试下一个分组
				logger.LogDebug(param.Ctx, "No available channel in group %s for model %s at priorityRetry %d, trying next group", autoGroup, param.ModelName, priorityRetry)
				// 重置状态以尝试下一个分组
				common.SetContextKey(param.Ctx, constant.ContextKeyAutoGroupIndex, i+1)
				common.SetContextKey(param.Ctx, constant.ContextKeyAutoGroupRetryIndex, 0)
				// Reset retry counter so outer loop can continue for next group
				// 重置重试计数器，以便外层循环可以为下一个分组继续
				param.SetRetry(0)
				continue
			}
			common.SetContextKey(param.Ctx, constant.ContextKeyAutoGroup, autoGroup)
			selectGroup = autoGroup
			logger.LogDebug(param.Ctx, "Auto selected group: %s", autoGroup)

			// Prepare state for next retry
			// 为下一次重试准备状态
			if crossGroupRetry && priorityRetry >= common.RetryTimes {
				// Current group has exhausted all retries, prepare to switch to next group
				// This request still uses current group, but next retry will use next group
				// 当前分组已用完所有重试次数，准备切换到下一个分组
				// 本次请求仍使用当前分组，但下次重试将使用下一个分组
				logger.LogDebug(param.Ctx, "Current group %s retries exhausted (priorityRetry=%d >= RetryTimes=%d), preparing switch to next group for next retry", autoGroup, priorityRetry, common.RetryTimes)
				common.SetContextKey(param.Ctx, constant.ContextKeyAutoGroupIndex, i+1)
				// Reset retry counter so outer loop can continue for next group
				// 重置重试计数器，以便外层循环可以为下一个分组继续
				param.SetRetry(0)
				param.ResetRetryNextTry()
			} else {
				// Stay in current group, save current state
				// 保持在当前分组，保存当前状态
				common.SetContextKey(param.Ctx, constant.ContextKeyAutoGroupIndex, i)
			}
			break
		}
	} else {
		channel, err = model.GetRandomSatisfiedChannel(param.TokenGroup, param.ModelName, param.GetRetry())
		if err != nil {
			return nil, param.TokenGroup, err
		}
	}
	return channel, selectGroup, nil
}

// selectChannelWithSmartRouter uses SmartRouter for health-aware channel selection
// 使用 SmartRouter 进行健康感知的渠道选择
func selectChannelWithSmartRouter(param *RetryParam, smartRouter SmartRouterIface) (*model.Channel, string, error) {
	// Get all channels for the model and group
	var channels []*model.Channel

	if param.TokenGroup == "auto" {
		userGroup := common.GetContextKeyString(param.Ctx, constant.ContextKeyUserGroup)
		autoGroups := GetUserAutoGroup(userGroup)
		if len(autoGroups) == 0 {
			return nil, param.TokenGroup, nil // No error, just no channels
		}

		// Collect channels from all auto groups using GetAllChannels
		allChannels, _ := model.GetAllChannels(0, 0, true, false)
		for _, ch := range allChannels {
			if ch.Status != 1 {
				continue
			}
			// Check if channel supports the model and any of the auto groups
			for _, group := range autoGroups {
				if model.IsChannelEnabledForGroupModel(group, param.ModelName, ch.Id) {
					channels = append(channels, ch)
					break
				}
			}
		}
	} else {
		// Get all channels and filter by group and model
		allChannels, err := model.GetAllChannels(0, 0, true, false)
		if err != nil {
			return nil, param.TokenGroup, err
		}
		for _, ch := range allChannels {
			if ch.Status != 1 {
				continue
			}
			if model.IsChannelEnabledForGroupModel(param.TokenGroup, param.ModelName, ch.Id) {
				channels = append(channels, ch)
			}
		}
	}

	if len(channels) == 0 {
		return nil, param.TokenGroup, nil // No error, just no channels
	}

	// Filter channels by priority (if retry is set)
	if param.GetRetry() > 0 {
		filtered := make([]*model.Channel, 0)
		for _, ch := range channels {
			if ch.Priority != nil && int(*ch.Priority) >= param.GetRetry() {
				filtered = append(filtered, ch)
			}
		}
		if len(filtered) > 0 {
			channels = filtered
		}
	}

	// Use SmartRouter to select based on health
	selected := smartRouter.SelectHealthyChannel(channels)
	if selected == nil {
		// All channels unhealthy, return nil to fall back to original logic
		return nil, param.TokenGroup, nil
	}

	logger.LogDebug(param.Ctx, "SmartRouter selected channel %d (health score: %.2f) for model %s",
		selected.Id, smartRouter.GetChannelHealthScore(selected.Id), param.ModelName)

	return selected, param.TokenGroup, nil
}
