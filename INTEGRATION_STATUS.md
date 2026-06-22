# New-API Fusion 整合状态深度分析报告

> 分析时间: 2026-06-20  
> 分析范围: 9router / AIClient2API / sub2api → new-api 整合版本

---

## 一、已整合功能现状（代码级验证）

### 1. Error Passthrough 规则系统

**整合状态**: ⚠️ 完整但未生效

**代码证据**:
- ✅ `model/error_passthrough_rule.go:13-30` - 完整的数据模型
- ✅ `model/error_passthrough_rule.go:98-109` - ErrorPassthroughService 服务
- ✅ `model/error_passthrough_rule.go:112-146` - ReloadRules() 缓存加载
- ✅ `model/error_passthrough_rule.go:149-184` - MatchRule() 匹配引擎
- ✅ `controller/error_passthrough.go` - CRUD API
- ✅ `web/default/src/.../error-passthrough-section.tsx` - 管理界面
- ❌ **`controller/relay.go` 中无任何调用** - 规则不会被应用

**验证方法**:
```bash
grep -rn "MatchRule\|ErrorPassthroughService" controller/relay.go
# 结果: 无匹配
```

**影响**: 配置了规则但请求错误时不会触发规则匹配

---

### 2. Channel Scoring 智能路由

**整合状态**: ✅ 完整且已接入

**代码证据**:
- ✅ `model/channel_score.go:12-21` - ChannelScore 数据结构
- ✅ `model/channel_score.go:58-95` - CalculateScore() 评分算法
- ✅ `model/channel_score.go:98-118` - RecordSuccess() 成功记录
- ✅ `model/channel_score.go:121-150` - RecordError() 错误记录 + 429退避
- ✅ `controller/relay_integration.go:18-35` - RecordChannelSuccess()
- ✅ `controller/relay_integration.go:37-54` - RecordChannelError()
- ✅ `controller/relay.go:226` - `RecordChannelSuccess(c, channel.Id)` 调用
- ✅ `controller/relay.go:235` - `RecordChannelError(c, channel.Id, newAPIError)` 调用
- ✅ `relay/smart_router_manager.go:34-40` - BuildFallbackChainsFromChannels()
- ⚠️ **评分数据仅用于记录，未实际影响 channel selection**

**验证方法**:
```bash
grep -rn "CalculateScore\|GetChannelScore" middleware/distributor.go
# 结果: 无匹配（评分未接入选择逻辑）
```

**影响**: 评分系统完整但只是"观察者"，不影响路由决策

---

### 3. Ops Alert 评估器

**整合状态**: ⚠️ 完整但未触发

**代码证据**:
- ✅ `model/ops_alert.go:12-23` - OpsAlert 数据模型
- ✅ `model/ops_alert.go:43-46` - AlertEvaluator 结构
- ✅ `model/ops_alert.go:79-116` - Evaluate() 评估逻辑
- ✅ `model/ops_alert.go:118-129` - createAlert() 告警创建
- ✅ `controller/ops_dashboard.go:24-32` - UpdateAlertThresholds API
- ❌ **`main.go` 中无定时调用 Evaluate()**
- ❌ **`relay.go` 中无数据喂给评估器**

**验证方法**:
```bash
grep -rn "\.Evaluate(" main.go controller/relay.go
# 结果: 无匹配
```

**影响**: 可以配置阈值但永远不会触发告警

---

### 4. Ops Trend 趋势统计

**整合状态**: ⚠️ 完整但未记录

**代码证据**:
- ✅ `model/ops_alert.go:140-143` - trendStore 内存存储
- ✅ `model/ops_alert.go:145-177` - RecordTrend() 记录逻辑
- ✅ `model/ops_alert.go:180-193` - GetTrends() 查询逻辑
- ✅ `controller/ops_dashboard.go:35-44` - GetOpsTrends API
- ❌ **`relay.go` 中无调用 RecordTrend()**

**验证方法**:
```bash
grep -rn "RecordTrend" controller/relay.go service/
# 结果: 无匹配（只在定义处）
```

**影响**: 趋势图永远为空

---

### 5. Prompt Replacement 系统提示替换

**整合状态**: ⚠️ 完整但未接入

**代码证据**:
- ✅ `controller/relay_integration.go:100-108` - PromptReplacementRule 定义
- ✅ `controller/relay_integration.go:113-134` - ApplyPromptReplacements()
- ✅ `controller/relay_integration.go:137-145` - SetPromptReplacementRules()
- ✅ `controller/fallback_chain.go:78-112` - UpdatePromptReplacements API
- ❌ **`relay/` 中无调用 ApplyPromptReplacements()**

**验证方法**:
```bash
grep -rn "ApplyPromptReplacements" relay/ controller/relay.go
# 结果: 无匹配
```

**影响**: 配置了替换规则但不会应用到请求中

---

### 6. Fallback Chain 降级链

**整合状态**: ⚠️ 完整但未接入

**代码证据**:
- ✅ `model/fallback_chain.go` - FallbackChainConfig 模型
- ✅ `controller/fallback_chain.go:16-22` - GetFallbackChainConfig API
- ✅ `controller/fallback_chain.go:22-44` - UpdateFallbackChainConfig API
- ✅ `controller/relay_integration.go:67-82` - CheckModelMapping()
- ✅ `controller/relay_integration.go:84-96` - GetFallbackChannelTypes()
- ✅ `relay/smart_router_manager.go:34-40` - BuildFallbackChainsFromChannels()
- ❌ **`relay.go` getChannel() 中无 fallback 逻辑**

**验证方法**:
```bash
grep -rn "CheckModelMapping\|GetFallbackChannelTypes" controller/relay.go
# 结果: 无匹配
```

**影响**: 配置了降级链但不会在主渠道失败时触发

---

### 7. Idempotency 幂等性

**整合状态**: ✅ 完整且已接入

**代码证据**:
- ✅ `middleware/idempotency.go:17-60` - IdempotencyStore 内存存储
- ✅ `middleware/idempotency.go:82-120` - Idempotency() 中间件
- ✅ `router/relay-router.go:73` - `relayV1Router.Use(middleware.Idempotency())`
- ⚠️ **纯内存实现，重启丢失**

**影响**: 正常工作，但进程重启后缓存清空（个人使用可接受）

---

### 8. WebSocket Realtime 支持

**整合状态**: ✅ 基础框架已接入

**代码证据**:
- ✅ `controller/relay.go:32` - `"github.com/gorilla/websocket"`
- ✅ `controller/relay.go:76` - `ws *websocket.Conn`
- ✅ `controller/relay.go:79-87` - WebSocket 升级逻辑
- ✅ `controller/relay.go:254-259` - upgrader 配置
- ✅ `relay/constant/relay_mode.go:90` - Realtime 模式识别
- ✅ `middleware/distributor.go:337-338` - Realtime 路径处理
- ⚠️ **缺少连接池、状态存储、HTTP-WS 桥接等高级功能**

**影响**: 基础 WebSocket 可用，但缺少 sub2api 的完整会话管理

---

### 9. Channel Monitor 渠道监控

**整合状态**: ✅ 完整

**代码证据**:
- ✅ `model/channel_monitor.go:10-30` - ChannelMonitor 模型
- ✅ `model/channel_monitor.go:45-58` - ChannelMonitorHistory 历史
- ✅ `model/channel_monitor.go:63-85` - CRUD 操作
- ✅ `service/channel_monitor.go:21-32` - StartChannelMonitorScheduler()
- ✅ `service/channel_monitor.go:34-55` - runScheduledChecks()
- ✅ `service/channel_monitor.go:57-91` - checkMonitor()
- ✅ `service/channel_monitor.go:100-155` - testChannelModel()
- ✅ `main.go:146` - `service.StartChannelMonitorScheduler()` 启动
- ⚠️ **缺少按天聚合的可用性计算**

**影响**: 监控和告警正常工作，但缺少历史可用性统计

---

### 10. Console Log SSE 推送

**整合状态**: ✅ 完整且已接入

**代码证据**:
- ✅ `controller/console_log_sse.go:23-42` - PublishConsoleLog()
- ✅ `controller/console_log_sse.go:45-87` - ConsoleLogStream() SSE
- ✅ `controller/console_log_sse.go:90-97` - GetConsoleLogBuffer()
- ✅ `controller/ops_dashboard.go:47-48` - GetConsoleLogs/StreamConsoleLogs
- ✅ `common/sys_log.go:17` - ConsoleLogPublisher 回调
- ✅ `common/sys_log.go:24-28` - SysLog() 调用 publisher
- ✅ `logger/logger.go:110-115` - logHelper() 调用 publisher
- ✅ `main.go:311` - `common.ConsoleLogPublisher = controller.PublishConsoleLog`

**影响**: 运维面板实时日志正常工作

---

## 二、整合缺口优先级

### 🔴 P0 - 必须修复（功能存在但完全不生效）

| # | 功能 | 问题 | 工作量 |
|---|---|---|---|
| 1 | **Error Passthrough** | MatchRule() 未被调用 | 中 (~150行) |
| 2 | **Ops Alert Evaluator** | Evaluate() 未被调用 | 小 (~50行) |
| 3 | **Ops Trend RecordTrend** | RecordTrend() 未被调用 | 小 (~30行) |
| 4 | **Prompt Replacement** | ApplyPromptReplacements() 未被调用 | 小 (~40行) |
| 5 | **Fallback Chain** | CheckModelMapping() 未被调用 | 中 (~200行) |

### 🟡 P1 - 高价值增强

| # | 功能 | 问题 | 工作量 |
|---|---|---|---|
| 6 | **Channel Scoring 接入路由** | CalculateScore() 未影响选择 | 中 (~150行) |
| 7 | **Channel Monitor 聚合统计** | 缺少每日可用性计算 | 中 (~100行) |

---

## 三、修复方案

### 修复 1: Error Passthrough 接入 relay 错误处理

**位置**: `controller/relay.go` 第 89-106 行的 defer 块

**当前代码**:
```go
defer func() {
    if newAPIError != nil {
        logger.LogError(c, fmt.Sprintf("relay error: %s", ...))
        newAPIError.SetMessage(common.MessageWithRequestId(...))
        switch relayFormat {
        case types.RelayFormatOpenAIRealtime:
            helper.WssError(c, ws, newAPIError.ToOpenAIError())
        // ...
        }
    }
}()
```

**需要添加**:
```go
defer func() {
    if newAPIError != nil {
        // === Error Passthrough 规则匹配 ===
        svc := model.GetErrorPassthroughService()
        platform := c.GetString("platform") // 需要在请求解析时设置
        rule := svc.MatchRule(platform, newAPIError.StatusCode, nil)
        if rule != nil {
            // 应用规则：替换状态码或消息体
            if !rule.PassthroughCode && rule.ResponseCode != nil {
                newAPIError.StatusCode = *rule.ResponseCode
            }
            if !rule.PassthroughBody && rule.CustomMessage != nil {
                newAPIError.SetMessage(*rule.CustomMessage)
            }
        }
        // === End Error Passthrough ===
        
        logger.LogError(c, ...)
        // ...
    }
}()
```

**额外需要**:
- `main.go` 启动时调用 `model.GetErrorPassthroughService().ReloadRules()`
- CRUD 操作后调用 `ReloadRules()` 刷新缓存

---

### 修复 2: Ops Alert Evaluator 定时触发

**位置**: `main.go` 初始化后

**需要添加**:
```go
// 启动 Ops Alert 评估定时任务
go func() {
    ticker := time.NewTicker(60 * time.Second) // 每分钟评估一次
    defer ticker.Stop()
    for range ticker.C {
        evaluateAllChannels()
    }
}()

func evaluateAllChannels() {
    channels, _ := model.GetAllChannels(0, 0, true, false)
    evaluator := model.GetAlertEvaluator()
    for _, ch := range channels {
        score := model.GetChannelScore(ch.Id)
        total := score.ErrorCount + score.SuccessCount
        if total == 0 {
            continue
        }
        errorRate := float64(score.ErrorCount) / float64(total)
        avgLatency := 0.0
        if score.SuccessCount > 0 {
            avgLatency = float64(score.TotalLatencyMs) / float64(score.SuccessCount)
        }
        evaluator.Evaluate(ch.Id, ch.Name, errorRate, avgLatency, score.Consecutive429)
    }
}
```

---

### 修复 3: Ops Trend RecordTrend 接入

**位置**: `controller/relay.go` 成功和失败路径

**成功路径** (第 226 行附近):
```go
RecordChannelSuccess(c, channel.Id)
// 添加:
startTime := c.GetTime("relay_start_time")
latencyMs := int64(0)
if !startTime.IsZero() {
    latencyMs = time.Since(startTime).Milliseconds()
}
model.RecordTrend(latencyMs, false, 0, 0, 0) // tokens/cost 可从 usage 获取
```

**失败路径** (第 247-251 行附近):
```go
if newAPIError != nil {
    gopool.Go(func() {
        perfmetrics.RecordRelaySample(relayInfo, false, 0)
        // 添加:
        model.RecordTrend(0, true, 0, 0, 0)
    })
}
```

---

### 修复 4: Prompt Replacement 接入

**位置**: `relay/` 中构建请求体的地方

**需要找到**: 在发送到上游之前处理 system prompt 的位置，然后调用:
```go
if prompt := request.GetSystemPrompt(); prompt != "" {
    newPrompt := ApplyPromptReplacements(prompt)
    request.SetSystemPrompt(newPrompt)
}
```

---

### 修复 5: Fallback Chain 接入 getChannel

**位置**: `controller/relay.go` 第 296-326 行的 getChannel()

**当前逻辑**:
```go
func getChannel(...) (*model.Channel, *types.NewAPIError) {
    channel, selectGroup, err := service.CacheGetRandomSatisfiedChannel(retryParam)
    // ...
}
```

**需要添加 fallback**:
```go
func getChannel(...) (*model.Channel, *types.NewAPIError) {
    // 1. 先检查模型映射
    targetModel, mapped := CheckModelMapping(info.OriginModelName)
    if mapped {
        info.OriginModelName = targetModel
    }
    
    // 2. 尝试获取主渠道
    channel, selectGroup, err := service.CacheGetRandomSatisfiedChannel(retryParam)
    if err != nil || channel == nil {
        // 3. 如果失败，尝试 fallback chain
        fallbackTypes := GetFallbackChannelTypes(channel.Type)
        for _, fbType := range fallbackTypes {
            retryParam.ChannelType = fbType
            channel, _, err = service.CacheGetRandomSatisfiedChannel(retryParam)
            if err == nil && channel != nil {
                break
            }
        }
    }
    // ...
}
```

---

### 修复 6: Channel Scoring 接入路由选择

**位置**: `middleware/distributor.go` 或 `service/channel.go` 的 channel selection 逻辑

**当前**: `CacheGetRandomSatisfiedChannel()` 随机选择

**需要**: 在候选 channel 列表中按评分排序，优先选择低分 channel

---

## 四、总结

### 代码质量评估

| 维度 | 评分 | 说明 |
|---|---|---|
| **模型完整性** | ✅ 95% | 所有数据模型都完整实现 |
| **API 完整性** | ✅ 90% | CRUD 接口完整 |
| **前端完整性** | ✅ 85% | 管理界面完整 |
| **核心接入** | ❌ 40% | 多个核心功能未接入 relay |

### 根因分析

整合者完成了：
1. ✅ 从 sub2api/9router/AIClient2API 移植了完整的模型和服务
2. ✅ 实现了 CRUD API 和前端管理界面
3. ❌ **但在 relay 核心流程中没有"接线"**

这导致了一个"看起来完整但实际不工作"的状态：
- 可以配置规则/阈值/映射
- 但这些配置永远不会被应用

### 下一步行动

**立即修复 P0**（约 2-3 小时）：
1. Error Passthrough 接入
2. Ops Alert 定时触发
3. Ops Trend 记录
4. Prompt Replacement 接入
5. Fallback Chain 接入

**后续优化 P1**（约 1-2 小时）：
6. Channel Scoring 接入路由选择
7. Channel Monitor 聚合统计
