# New API Fusion - 待修复任务清单

> 最后更新: 2026-06-20  
> 状态: 🔴 未开始 | 🟡 进行中 | ✅ 已完成 | ⏸️ 暂停

---

## 📊 任务统计

- **P0 严重**: 0/5 完成
- **P1 高优先级**: 0/5 完成  
- **P2 中优先级**: 0/6 完成
- **总计**: 0/16 完成

---

## 🚨 P0 - 严重问题（功能完全不可用）

### [🔴] #1 CLI 工具配置 - API 路径 404
**组件**: `cli-tools-section.tsx`  
**问题**: 前端调用 `/api/user/tool-configs` 但后端路由是 `/api/tool-configs`  
**影响**: CLI 工具页面完全无法加载

**修复步骤**:
- [ ] 修改前端 API 路径: `/api/user/tool-configs` → `/api/tool-configs`
- [ ] 修改前端 API 路径: `/api/user/tool-configs/generate` → `/api/tool-configs/generate`
- [ ] 测试: 页面加载显示工具列表
- [ ] 测试: 生成配置功能正常

---

### [🔴] #2 集成面板 - 三组 API 全部 404
**组件**: `integrations-section.tsx`  
**问题**: 
- `/api/user/dingtalk-config` → `/api/dingtalk-config`
- `/api/user/wecom-config` → `/api/wecom-config`
- `/api/user/balance-notify` → `/api/balance-notify`

**影响**: 钉钉/企微 OAuth 和余额通知功能完全不可用

**修复步骤**:
- [ ] 修改钉钉配置 API 路径
- [ ] 修改企微配置 API 路径
- [ ] 修改余额通知 API 路径
- [ ] 测试: 钉钉 OAuth 配置保存/加载
- [ ] 测试: 企微 OAuth 配置保存/加载
- [ ] 测试: 余额通知配置保存/加载

---

### [🔴] #3 高级运维 - 告警阈值保存 404
**组件**: `advanced-section.tsx`  
**问题**: 前端调用 `/api/user/ops/alert-thresholds` 但后端路由是 `/api/ops/alert-thresholds`  
**影响**: 告警阈值无法保存

**修复步骤**:
- [ ] 修改前端 API 路径
- [ ] 测试: 保存告警阈值
- [ ] 测试: 刷新后配置仍然存在

---

### [🔴] #4 高级运维 - uTLS 保存格式错误
**组件**: `advanced-section.tsx`  
**问题**: 
```typescript
// 错误: 批量格式
updateSystemOption({ options: { utls_enabled: 'true', utls_fingerprint: 'chrome' } })

// 正确: 单个键值对
updateSystemOption({ key: 'utls_enabled', value: 'true' })
updateSystemOption({ key: 'utls_fingerprint', value: 'chrome' })
```

**影响**: uTLS 配置无法保存，保存时报错

**修复步骤**:
- [ ] 修改前端保存逻辑为两次单独调用
- [ ] 后端 `UpdateOption` 添加 `utls_enabled` 和 `utls_fingerprint` 处理逻辑
- [ ] 后端启动时从数据库恢复 uTLS 配置
- [ ] 测试: 启用 uTLS 并选择指纹
- [ ] 测试: 重启容器后配置仍然存在

---

### [🔴] #5 五个 CRUD 组件使用裸 fetch() - 缺少认证
**组件**: 
- `channel-monitor-section.tsx`
- `proxy-pool-section.tsx`
- `error-passthrough-section.tsx`
- `websearch-cleanup-section.tsx` (WebSearch + UsageCleanup)

**问题**: 使用原生 `fetch()` 而不是 `api.get()`/`api.post()`，缺少认证 token  
**影响**: 所有请求返回 401 Unauthorized

**修复步骤**:
- [ ] `channel-monitor-section.tsx`: 替换所有 `fetch()` 为 `api` 调用
- [ ] `proxy-pool-section.tsx`: 替换所有 `fetch()` 为 `api` 调用
- [ ] `error-passthrough-section.tsx`: 替换所有 `fetch()` 为 `api` 调用
- [ ] `websearch-cleanup-section.tsx`: 替换所有 `fetch()` 为 `api` 调用
- [ ] 测试: 每个组件的 CRUD 操作（增删改查）

---

## ⚠️ P1 - 高优先级问题（功能异常）

### [🔴] #6 四个新组件 - i18n 翻译大面积缺失
**组件**: 
- `ops-dashboard-section.tsx` - "运维面板"、"实时日志"、"等待日志..." 等
- `cli-tools-section.tsx` - "CLI 工具配置生成器"、"生成配置" 等
- `advanced-section.tsx` - "高级运维"、"节点评分" 等
- `integrations-section.tsx` - "钉钉 OAuth"、"企业微信" 等

**问题**: 大量硬编码中文，未使用 `t()` 函数  
**影响**: 英文界面显示中文，国际化不完整

**修复步骤**:
- [ ] 提取 `ops-dashboard-section.tsx` 所有中文文本
- [ ] 提取 `cli-tools-section.tsx` 所有中文文本
- [ ] 提取 `advanced-section.tsx` 所有中文文本
- [ ] 提取 `integrations-section.tsx` 所有中文文本
- [ ] 在 `zh.json` 和 `en.json` 中添加翻译
- [ ] 测试: 切换语言后文本正确显示

---

### [🔴] #7 Ops Dashboard - 趋势数据来源错误
**组件**: `ops-dashboard-section.tsx`  
**问题**: 
```typescript
// 错误: 使用模型聚合数据
const res = await api.get('/api/perf-metrics/summary')
// 返回的是模型维度的统计数据，不是时间序列

// 正确: 使用时间序列接口
const res = await api.get('/api/ops/trends')
// 返回的是按时间排序的趋势数据
```

**影响**: 趋势图显示错误，所有数据点时间戳相同

**修复步骤**:
- [ ] 修改前端 API 调用为 `/api/ops/trends`
- [ ] 调整前端数据解析逻辑
- [ ] 测试: 趋势图显示正确的时间序列

---

### [🔴] #8 Ops Dashboard - SSE 日志流无认证
**组件**: `ops-dashboard-section.tsx`  
**问题**: 
```typescript
const es = new EventSource('/api/user/console-logs/stream')
```
`EventSource` API 不支持自定义 headers，无法传递认证 token

**影响**: SSE 连接返回 401，日志流不工作

**修复步骤**:
- [ ] 方案 A: 改用 `fetch()` + ReadableStream 手动实现 SSE
- [ ] 方案 B: 改用轮询 `/api/ops/console-logs` 接口
- [ ] 方案 C: 后端添加 cookie 认证支持（需要修改认证中间件）
- [ ] 测试: 日志流正常显示

---

### [🔴] #9 Channel Monitor - 编辑表单缺少字段
**组件**: `channel-monitor-section.tsx`  
**问题**: 接口定义有 `note` 字段，但表单没有显示和编辑这个字段  
**影响**: 无法添加/编辑监控任务的备注信息

**修复步骤**:
- [ ] 在表单中添加 `note` 输入框
- [ ] 确保编辑时回填 `note` 值
- [ ] 测试: 创建带备注的监控任务
- [ ] 测试: 编辑备注

---

### [🔴] #10 Error Passthrough - JSON 字段无验证
**组件**: `error-passthrough-section.tsx`  
**问题**: 
```typescript
<Input value={errorCodes} onChange={e => setErrorCodes(e.target.value)} 
       placeholder="[429, 503]" />
```
用户需要手动输入 JSON 数组，但没有格式验证

**影响**: 
- 输入格式错误时保存失败但没有友好提示
- 编辑时显示原始 JSON 字符串，不友好

**修复步骤**:
- [ ] 添加 JSON 格式验证（保存前检查）
- [ ] 添加格式错误提示
- [ ] 优化编辑时的显示格式（可选：使用 tag input）
- [ ] 测试: 输入正确 JSON 保存成功
- [ ] 测试: 输入错误 JSON 显示错误提示

---

## 📝 P2 - 中优先级问题（体验优化）

### [🔴] #11 四个组件 - 混用原生 select 和 shadcn Select
**组件**: 
- `advanced-section.tsx` - 原生 `<select>`
- `cli-tools-section.tsx` - 原生 `<select>`
- `error-passthrough-section.tsx` - 原生 `<select>`
- `websearch-cleanup-section.tsx` - 原生 `<select>`

**问题**: 与项目其他组件风格不统一  
**影响**: 原生 select 不支持暗色主题、键盘导航、搜索过滤

**修复步骤**:
- [ ] 将所有原生 `<select>` 替换为 shadcn `<Select>` 组件
- [ ] 测试: 暗色主题下样式正确
- [ ] 测试: 键盘导航正常

---

### [🔴] #12 四个组件 - 组件容器不统一
**组件**: 
- 使用 `<Card>`: `rtk-settings-section.tsx`, `cli-tools-section.tsx`, `advanced-section.tsx`, `ops-dashboard-section.tsx`
- 使用 `<SettingsSection>`: `channel-monitor-section.tsx`, `proxy-pool-section.tsx`, `error-passthrough-section.tsx`

**问题**: 同一设置页面视觉层次不一致  
**影响**: UI 不一致

**修复步骤**:
- [ ] 统一所有组件使用 `<SettingsSection>` 或 `<Card>`
- [ ] 测试: 所有组件视觉风格一致

---

### [🔴] #13 五个组件 - 缺少 loading 状态
**组件**: 
- `channel-monitor-section.tsx`
- `proxy-pool-section.tsx`
- `error-passthrough-section.tsx`
- `websearch-cleanup-section.tsx` (WebSearch + UsageCleanup)

**问题**: 数据加载时没有任何 loading 指示  
**影响**: 用户不知道是在加载还是没有数据

**修复步骤**:
- [ ] 添加 `loading` state
- [ ] 数据加载时显示 loading spinner 或 skeleton
- [ ] 测试: 加载过程中显示 loading 状态

---

### [🔴] #14 WebSearch - API Key 明文显示
**组件**: `websearch-cleanup-section.tsx`  
**问题**: 
```typescript
<TableCell className="font-mono text-xs">{p.api_key}</TableCell>
```
API Key 直接明文展示

**影响**: 安全隐患，任何人打开页面都能看到 API Key

**修复步骤**:
- [ ] 前端: 显示时脱敏（如 `sk-****...****`）
- [ ] 后端: 返回时脱敏（可选，更安全）
- [ ] 测试: 列表中 API Key 显示为脱敏格式

---

### [🔴] #15 后端路由 - GetOpsAlerts 重复注册
**文件**: `router/api-router.go`  
**问题**: 
```go
// 第 224 行 (adminRoute)
adminRoute.GET("/ops/alerts", controller.GetOpsAlerts)

// 第 510 行 (opsAlertsRoute)
opsAlertsRoute.GET("", controller.GetOpsAlerts)
```
同一 handler 注册了两次

**影响**: 代码冗余，可能造成混淆

**修复步骤**:
- [ ] 删除重复的路由注册（保留一个）
- [ ] 更新前端调用对应的路由
- [ ] 测试: 告警列表正常加载

---

### [🔴] #16 UsageCleanup - filters 字段格式可能不匹配
**组件**: `websearch-cleanup-section.tsx`  
**问题**: 
```typescript
body: JSON.stringify({ 
  filters: { start_time: ..., end_time: ... } 
})
```
前端传对象，但后端模型可能是 string 类型

**影响**: 可能导致反序列化失败

**修复步骤**:
- [ ] 检查后端 `UsageCleanupTask` 模型的 `filters` 字段类型
- [ ] 如果是 string，前端需要 `JSON.stringify(filters)` 再传
- [ ] 测试: 创建清理任务成功

---

## 🔧 后端待实现功能

### [🔴] uTLS 配置持久化
**文件**: `controller/option.go`  
**问题**: uTLS 配置只保存在内存，重启后丢失

**修复步骤**:
- [ ] `UpdateOption` 添加 `utls_enabled` case
- [ ] `UpdateOption` 添加 `utls_fingerprint` case
- [ ] 保存到数据库: `model.SaveConfig("utls_enabled", value)`
- [ ] 保存到数据库: `model.SaveConfig("utls_fingerprint", value)`
- [ ] 调用 `service.EnableUTLS(fingerprint)` 立即生效
- [ ] `main.go` 启动时从数据库恢复 uTLS 配置

---

### [🔴] Console Log 推送机制
**文件**: `controller/console_log_sse.go`, `logger/logger.go`  
**问题**: `PublishConsoleLog()` 已定义但从未调用

**修复步骤**:
- [ ] 在 relay middleware 中调用 `PublishConsoleLog()`
- [ ] 或在 logger 中添加 hook 自动推送
- [ ] 测试: 发起 API 请求后运维面板显示日志

---

## 📋 测试清单

### P0 测试
- [ ] CLI 工具页面加载并生成配置
- [ ] 钉钉/企微 OAuth 配置保存/加载
- [ ] 余额通知配置保存/加载
- [ ] 告警阈值保存并刷新后仍存在
- [ ] uTLS 启用并选择指纹，重启后配置仍在
- [ ] Channel Monitor CRUD 操作
- [ ] Proxy Pool CRUD 操作
- [ ] Error Passthrough CRUD 操作
- [ ] WebSearch CRUD 操作
- [ ] Usage Cleanup 创建任务

### P1 测试
- [ ] 切换语言后所有新增组件文本正确
- [ ] Ops Dashboard 趋势图显示正确的时间序列
- [ ] Ops Dashboard 日志流正常显示
- [ ] Channel Monitor 编辑备注
- [ ] Error Passthrough JSON 格式验证

### P2 测试
- [ ] 所有 select 组件在暗色主题下样式正确
- [ ] 所有组件视觉风格一致
- [ ] 所有组件加载时显示 loading 状态
- [ ] WebSearch API Key 脱敏显示
- [ ] 无重复路由注册

---

## 📝 备注

### 已完成的工作
- ✅ 完成所有问题的分析和定位
- ✅ 识别了 16 个问题，按优先级分类
- ✅ 检查了前后端代码、路由、数据流

### 下一步
1. 按 P0 → P1 → P2 顺序修复
2. 每修复一个问题立即测试
3. 全部完成后重新构建前端
4. 重建 Docker 镜像
5. 部署到本地和远程服务器
6. 推送到 GitHub（force push，不保留历史）

---

## 🔗 相关文件

### 前端组件
- `web/default/src/features/system-settings/operations/advanced-section.tsx`
- `web/default/src/features/system-settings/operations/cli-tools-section.tsx`
- `web/default/src/features/system-settings/operations/ops-dashboard-section.tsx`
- `web/default/src/features/system-settings/operations/integrations-section.tsx`
- `web/default/src/features/system-settings/integrations/channel-monitor-section.tsx`
- `web/default/src/features/system-settings/integrations/proxy-pool-section.tsx`
- `web/default/src/features/system-settings/integrations/error-passthrough-section.tsx`
- `web/default/src/features/system-settings/integrations/websearch-cleanup-section.tsx`

### 后端文件
- `router/api-router.go`
- `controller/option.go`
- `controller/console_log_sse.go`
- `controller/ops_dashboard.go`
- `controller/tool_configs.go`
- `logger/logger.go`
- `service/http_client.go`

### 配置文件
- `web/default/src/i18n/locales/zh.json`
- `web/default/src/i18n/locales/en.json`
