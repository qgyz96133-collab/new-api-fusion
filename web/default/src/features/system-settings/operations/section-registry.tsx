/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
import { SystemBehaviorSection } from '../general/system-behavior-section'
import { EmailSettingsSection } from '../integrations/email-settings-section'
import { MonitoringSettingsSection } from '../integrations/monitoring-settings-section'
import { WorkerSettingsSection } from '../integrations/worker-settings-section'
import { LogSettingsSection } from '../maintenance/log-settings-section'
import { PerformanceSection } from '../maintenance/performance-section'
import { RTKSettingsSection } from '../integrations/rtk-settings-section'
import { UpdateCheckerSection } from '../maintenance/update-checker-section'
import { AdvancedSection } from './advanced-section'
import { CliToolsSection } from './cli-tools-section'
import { OpsDashboardSection } from './ops-dashboard-section'
import { IntegrationsSection } from './integrations-section'
import { ProxyPoolSection } from '../integrations/proxy-pool-section'
import { ErrorPassthroughSection } from '../integrations/error-passthrough-section'
import { WebSearchSection } from '../integrations/websearch-cleanup-section'
import type { OperationsSettings } from '../types'
import { createSectionRegistry } from '../utils/section-registry'

const OPERATIONS_SECTIONS = [
  {
    id: 'behavior',
    titleKey: 'System Behavior',
    build: (settings: OperationsSettings) => (
      <SystemBehaviorSection
        defaultValues={{
          RetryTimes: settings.RetryTimes,
          DefaultCollapseSidebar: settings.DefaultCollapseSidebar,
          DemoSiteEnabled: settings.DemoSiteEnabled,
          SelfUseModeEnabled: settings.SelfUseModeEnabled,
        }}
      />
    ),
  },
  {
    id: 'monitoring',
    titleKey: 'Monitoring & Alerts',
    build: (settings: OperationsSettings) => (
      <MonitoringSettingsSection
        defaultValues={{
          ChannelDisableThreshold: settings.ChannelDisableThreshold,
          QuotaRemindThreshold: settings.QuotaRemindThreshold,
          AutomaticDisableChannelEnabled:
            settings.AutomaticDisableChannelEnabled,
          AutomaticEnableChannelEnabled: settings.AutomaticEnableChannelEnabled,
          AutomaticDisableKeywords: settings.AutomaticDisableKeywords,
          AutomaticDisableStatusCodes: settings.AutomaticDisableStatusCodes,
          AutomaticRetryStatusCodes: settings.AutomaticRetryStatusCodes,
          'monitor_setting.auto_test_channel_enabled':
            settings['monitor_setting.auto_test_channel_enabled'],
          'monitor_setting.auto_test_channel_minutes':
            settings['monitor_setting.auto_test_channel_minutes'],
        }}
      />
    ),
  },
  {
    id: 'email',
    titleKey: 'SMTP Email',
    build: (settings: OperationsSettings) => (
      <EmailSettingsSection
        defaultValues={{
          SMTPServer: settings.SMTPServer,
          SMTPPort: settings.SMTPPort,
          SMTPAccount: settings.SMTPAccount,
          SMTPFrom: settings.SMTPFrom,
          SMTPToken: settings.SMTPToken,
          SMTPSSLEnabled: settings.SMTPSSLEnabled,
          SMTPForceAuthLogin: settings.SMTPForceAuthLogin,
        }}
      />
    ),
  },
  {
    id: 'worker',
    titleKey: 'Worker Proxy',
    build: (settings: OperationsSettings) => (
      <WorkerSettingsSection
        defaultValues={{
          WorkerUrl: settings.WorkerUrl,
          WorkerValidKey: settings.WorkerValidKey,
          WorkerAllowHttpImageRequestEnabled:
            settings.WorkerAllowHttpImageRequestEnabled,
        }}
      />
    ),
  },
  {
    id: 'logs',
    titleKey: 'Log Maintenance',
    build: (settings: OperationsSettings) => (
      <LogSettingsSection
        defaultEnabled={Boolean(settings.LogConsumeEnabled)}
      />
    ),
  },
  {
    id: 'performance',
    titleKey: 'Performance',
    build: (settings: OperationsSettings) => (
      <PerformanceSection
        defaultValues={{
          'performance_setting.disk_cache_enabled':
            settings['performance_setting.disk_cache_enabled'] ?? false,
          'performance_setting.disk_cache_threshold_mb':
            settings['performance_setting.disk_cache_threshold_mb'] ?? 10,
          'performance_setting.disk_cache_max_size_mb':
            settings['performance_setting.disk_cache_max_size_mb'] ?? 1024,
          'performance_setting.disk_cache_path':
            settings['performance_setting.disk_cache_path'] ?? '',
          'performance_setting.monitor_enabled':
            settings['performance_setting.monitor_enabled'] ?? false,
          'performance_setting.monitor_cpu_threshold':
            settings['performance_setting.monitor_cpu_threshold'] ?? 90,
          'performance_setting.monitor_memory_threshold':
            settings['performance_setting.monitor_memory_threshold'] ?? 90,
          'performance_setting.monitor_disk_threshold':
            settings['performance_setting.monitor_disk_threshold'] ?? 95,
          'perf_metrics_setting.enabled':
            settings['perf_metrics_setting.enabled'] ?? true,
          'perf_metrics_setting.flush_interval':
            settings['perf_metrics_setting.flush_interval'] ?? 5,
          'perf_metrics_setting.bucket_time':
            settings['perf_metrics_setting.bucket_time'] ?? 'hour',
          'perf_metrics_setting.retention_days':
            settings['perf_metrics_setting.retention_days'] ?? 0,
        }}
      />
    ),
  },
  {
    id: 'rtk',
    titleKey: 'RTK Token Saver',
    build: (settings: OperationsSettings) => (
      <RTKSettingsSection
        defaultValues={{
          'rtk_setting.rtk_enabled': settings['rtk_setting.rtk_enabled'] ?? true,
          'rtk_setting.rtk_min_tokens': settings['rtk_setting.rtk_min_tokens'] ?? 100,
          'rtk_setting.rtk_max_tokens': settings['rtk_setting.rtk_max_tokens'] ?? 50000,
          'rtk_setting.rtk_compression_level': settings['rtk_setting.rtk_compression_level'] ?? 2,
          'rtk_setting.caveman_enabled': settings['rtk_setting.caveman_enabled'] ?? false,
          'rtk_setting.caveman_mode_level': settings['rtk_setting.caveman_mode_level'] ?? 0,
          'rtk_setting.caveman_min_tokens': settings['rtk_setting.caveman_min_tokens'] ?? 200,
          'rtk_setting.enable_tool_call_validation': settings['rtk_setting.enable_tool_call_validation'] ?? true,
          'rtk_setting.enable_orphan_tool_fix': settings['rtk_setting.enable_orphan_tool_fix'] ?? true,
          'rtk_setting.enable_gemini_schema_cleaning': settings['rtk_setting.enable_gemini_schema_cleaning'] ?? true,
          'rtk_setting.enable_claude_normalization': settings['rtk_setting.enable_claude_normalization'] ?? true,
          'rtk_setting.enable_remote_image_fetch': settings['rtk_setting.enable_remote_image_fetch'] ?? true,
        }}
      />
    ),
  },
  {
    id: 'update-checker',
    titleKey: 'System maintenance',
    build: (
      _settings: OperationsSettings,
      currentVersion?: string | null,
      startTime?: number | null
    ) => (
      <UpdateCheckerSection
        currentVersion={currentVersion}
        startTime={startTime}
      />
    ),
  },
  {
    id: 'advanced',
    titleKey: 'Advanced Ops',
    build: () => <AdvancedSection />,
  },
  {
    id: 'cli-tools',
    titleKey: 'CLI Tools',
    build: () => <CliToolsSection />,
  },
  {
    id: 'ops-dashboard',
    titleKey: 'Ops Dashboard',
    build: () => <OpsDashboardSection />,
  },
  {
    id: 'integrations',
    titleKey: 'Integrations',
    build: () => <IntegrationsSection />,
  },
  {
    id: 'proxy-pool' as const,
    titleKey: 'Proxy Pool',
    build: () => <ProxyPoolSection />,
  },
  {
    id: 'error-passthrough' as const,
    titleKey: 'Error Passthrough',
    build: () => <ErrorPassthroughSection />,
  },
  {
    id: 'websearch' as const,
    titleKey: 'Web Search',
    build: () => <WebSearchSection />,
  },
] as const

export type OperationsSectionId = (typeof OPERATIONS_SECTIONS)[number]['id']

const operationsRegistry = createSectionRegistry<
  OperationsSectionId,
  OperationsSettings,
  [string | null | undefined, number | null | undefined]
>({
  sections: OPERATIONS_SECTIONS,
  defaultSection: 'behavior',
  basePath: '/system-settings/operations',
  urlStyle: 'path',
})

export const OPERATIONS_SECTION_IDS = operationsRegistry.sectionIds
export const OPERATIONS_DEFAULT_SECTION = operationsRegistry.defaultSection
export const getOperationsSectionNavItems =
  operationsRegistry.getSectionNavItems
export const getOperationsSectionContent = operationsRegistry.getSectionContent
export const getOperationsSectionMeta = operationsRegistry.getSectionMeta
