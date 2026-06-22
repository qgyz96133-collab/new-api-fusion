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
import { useState, useEffect, useCallback } from 'react'
import { useTranslation } from 'react-i18next'
import { useQueryClient } from '@tanstack/react-query'
import { toast } from 'sonner'
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'
import { Switch } from '@/components/ui/switch'
import { Label } from '@/components/ui/label'
import { Input } from '@/components/ui/input'
import { Separator } from '@/components/ui/separator'
import { Button } from '@/components/ui/button'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { updateSystemOption } from '@/features/system-settings/api'

type RTKSettingsProps = {
  defaultValues: {
    'rtk_setting.rtk_enabled': boolean
    'rtk_setting.rtk_min_tokens': number
    'rtk_setting.rtk_max_tokens': number
    'rtk_setting.rtk_compression_level': number
    'rtk_setting.caveman_enabled': boolean
    'rtk_setting.caveman_mode_level': number
    'rtk_setting.caveman_min_tokens': number
    'rtk_setting.enable_tool_call_validation': boolean
    'rtk_setting.enable_orphan_tool_fix': boolean
    'rtk_setting.enable_gemini_schema_cleaning': boolean
    'rtk_setting.enable_claude_normalization': boolean
    'rtk_setting.enable_remote_image_fetch': boolean
  }
}

const COMPRESSION_LEVELS = [
  { value: 0, label: '关闭 (0)' },
  { value: 1, label: '轻量 10-20% (1)' },
  { value: 2, label: '中等 20-40% (2)' },
  { value: 3, label: '强力 40-60% (3)' },
  { value: 4, label: '激进 60-80% (4)' },
  { value: 5, label: '极限 80-90% (5)' },
  { value: 6, label: '最大 90%+ (6)' },
]

const CAVEMAN_LEVELS = [
  { value: 0, label: '关闭 (0)' },
  { value: 1, label: '轻量 (1)' },
  { value: 2, label: '完整 (2)' },
  { value: 3, label: '极限 (3)' },
  { value: 4, label: '文言轻量 (4)' },
  { value: 5, label: '文言完整 (5)' },
  { value: 6, label: '文言极限 (6)' },
]

export function RTKSettingsSection({ defaultValues }: RTKSettingsProps) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const [localSettings, setLocalSettings] = useState(defaultValues)
  const [savedSettings, setSavedSettings] = useState(defaultValues)
  const [isSaving, setIsSaving] = useState(false)

  useEffect(() => {
    setLocalSettings(defaultValues)
    setSavedSettings(defaultValues)
  }, [defaultValues])

  const hasChanges = JSON.stringify(localSettings) !== JSON.stringify(savedSettings)

  const updateLocal = useCallback((key: string, value: any) => {
    setLocalSettings(prev => ({ ...prev, [key]: value }))
  }, [])

  const handleSaveAll = async () => {
    setIsSaving(true)
    try {
      const changedKeys: string[] = []
      for (const key of Object.keys(localSettings) as Array<keyof typeof localSettings>) {
        if (localSettings[key] !== savedSettings[key]) {
          changedKeys.push(key)
        }
      }

      if (changedKeys.length === 0) {
        toast.info(t('没有需要保存的更改'))
        setIsSaving(false)
        return
      }

      for (const key of changedKeys) {
        const res = await updateSystemOption({ key, value: localSettings[key] })
        if (!res.success) {
          throw new Error(`保存 ${key} 失败: ${res.message || '未知错误'}`)
        }
      }

      setSavedSettings({ ...localSettings })
      queryClient.invalidateQueries({ queryKey: ['system-options'] })
      toast.success(t('保存成功，已更新 {count} 项设置', { count: changedKeys.length }))
    } catch (error: any) {
      toast.error(t('保存失败: ') + (error.message || t('请重试')))
    } finally {
      setIsSaving(false)
    }
  }

  return (
    <Card>
      <CardHeader>
        <CardTitle>{t('RTK Token 节省器')}</CardTitle>
        <CardDescription>
          {t('配置 RTK token 压缩和 Caveman 模式设置')}
        </CardDescription>
      </CardHeader>
      <CardContent className="space-y-6">
        {/* RTK Compression Section */}
        <div className="space-y-4">
          <h3 className="text-lg font-medium">{t('RTK 压缩')}</h3>

          <div className="flex items-center justify-between">
            <div className="space-y-0.5">
              <Label htmlFor="rtk-enabled">{t('启用 RTK 压缩')}</Label>
              <p className="text-sm text-muted-foreground">
                {t('压缩工具输出以节省 30-70% tokens')}
              </p>
            </div>
            <Switch
              id="rtk-enabled"
              checked={localSettings['rtk_setting.rtk_enabled']}
              onCheckedChange={(checked) => updateLocal('rtk_setting.rtk_enabled', checked)}
              disabled={isSaving}
            />
          </div>

          {localSettings['rtk_setting.rtk_enabled'] && (
            <>
              <div className="space-y-2">
                <Label htmlFor="min-tokens">{t('最小压缩 Token 数')}</Label>
                <Input
                  id="min-tokens"
                  type="number"
                  min={0}
                  max={100000}
                  value={localSettings['rtk_setting.rtk_min_tokens']}
                  onChange={(e) => updateLocal('rtk_setting.rtk_min_tokens', parseInt(e.target.value) || 0)}
                  disabled={isSaving}
                />
                <p className="text-xs text-muted-foreground">
                  {t('仅压缩超过此数量的工具输出')}
                </p>
              </div>

              <div className="space-y-2">
                <Label htmlFor="max-tokens">{t('最大 Token 阈值')}</Label>
                <Input
                  id="max-tokens"
                  type="number"
                  min={0}
                  max={500000}
                  value={localSettings['rtk_setting.rtk_max_tokens']}
                  onChange={(e) => updateLocal('rtk_setting.rtk_max_tokens', parseInt(e.target.value) || 0)}
                  disabled={isSaving}
                />
                <p className="text-xs text-muted-foreground">
                  {t('跳过超过此大小的工具输出压缩')}
                </p>
              </div>

              <div className="space-y-2">
                <Label>{t('压缩等级')}</Label>
                <Select
                  value={String(localSettings['rtk_setting.rtk_compression_level'])}
                  onValueChange={(val) => updateLocal('rtk_setting.rtk_compression_level', Number(val))}
                  disabled={isSaving}
                >
                  <SelectTrigger className="w-full">
                    <SelectValue placeholder={t('选择压缩等级')} />
                  </SelectTrigger>
                  <SelectContent>
                    {COMPRESSION_LEVELS.map((level) => (
                      <SelectItem key={level.value} value={String(level.value)}>
                        {level.label}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </div>
            </>
          )}
        </div>

        <Separator />

        {/* Caveman Mode Section */}
        <div className="space-y-4">
          <h3 className="text-lg font-medium">{t('Caveman 模式')}</h3>

          <div className="flex items-center justify-between">
            <div className="space-y-0.5">
              <Label htmlFor="caveman-enabled">{t('启用 Caveman 模式')}</Label>
              <p className="text-sm text-muted-foreground">
                {t('简化 LLM 输出风格以减少 token 消耗')}
              </p>
            </div>
            <Switch
              id="caveman-enabled"
              checked={localSettings['rtk_setting.caveman_enabled']}
              onCheckedChange={(checked) => updateLocal('rtk_setting.caveman_enabled', checked)}
              disabled={isSaving}
            />
          </div>

          {localSettings['rtk_setting.caveman_enabled'] && (
            <>
              <div className="space-y-2">
                <Label>{t('Caveman 强度')}</Label>
                <Select
                  value={String(localSettings['rtk_setting.caveman_mode_level'])}
                  onValueChange={(val) => updateLocal('rtk_setting.caveman_mode_level', Number(val))}
                  disabled={isSaving}
                >
                  <SelectTrigger className="w-full">
                    <SelectValue placeholder={t('选择 Caveman 强度')} />
                  </SelectTrigger>
                  <SelectContent>
                    {CAVEMAN_LEVELS.map((level) => (
                      <SelectItem key={level.value} value={String(level.value)}>
                        {level.label}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </div>

              <div className="space-y-2">
                <Label htmlFor="caveman-min-tokens">{t('Caveman 最小触发 Token 数')}</Label>
                <Input
                  id="caveman-min-tokens"
                  type="number"
                  min={0}
                  max={100000}
                  value={localSettings['rtk_setting.caveman_min_tokens']}
                  onChange={(e) => updateLocal('rtk_setting.caveman_min_tokens', parseInt(e.target.value) || 0)}
                  disabled={isSaving}
                />
                <p className="text-xs text-muted-foreground">
                  {t('仅当输出超过此数量时启用 Caveman 模式')}
                </p>
              </div>
            </>
          )}
        </div>

        <Separator />

        {/* Translation Fixes Section */}
        <div className="space-y-4">
          <h3 className="text-lg font-medium">{t('翻译修复')}</h3>
          <p className="text-sm text-muted-foreground">
            {t('修复常见兼容性问题')}
          </p>

          <div className="flex items-center justify-between">
            <div className="space-y-0.5">
              <Label>{t('Tool Call ID 验证')}</Label>
              <p className="text-xs text-muted-foreground">
                {t('修复 OpenAI tool call ID 格式问题')}
              </p>
            </div>
            <Switch
              checked={localSettings['rtk_setting.enable_tool_call_validation']}
              onCheckedChange={(checked) => updateLocal('rtk_setting.enable_tool_call_validation', checked)}
              disabled={isSaving}
            />
          </div>

          <div className="flex items-center justify-between">
            <div className="space-y-0.5">
              <Label>{t('孤立 Tool 修复')}</Label>
              <p className="text-xs text-muted-foreground">
                {t('修复孤立的 tool_calls 响应')}
              </p>
            </div>
            <Switch
              checked={localSettings['rtk_setting.enable_orphan_tool_fix']}
              onCheckedChange={(checked) => updateLocal('rtk_setting.enable_orphan_tool_fix', checked)}
              disabled={isSaving}
            />
          </div>

          <div className="flex items-center justify-between">
            <div className="space-y-0.5">
              <Label>{t('Gemini Schema 清理')}</Label>
              <p className="text-xs text-muted-foreground">
                {t('清理 Gemini API 的 schema 兼容性问题')}
              </p>
            </div>
            <Switch
              checked={localSettings['rtk_setting.enable_gemini_schema_cleaning']}
              onCheckedChange={(checked) => updateLocal('rtk_setting.enable_gemini_schema_cleaning', checked)}
              disabled={isSaving}
            />
          </div>

          <div className="flex items-center justify-between">
            <div className="space-y-0.5">
              <Label>{t('Claude 规范化')}</Label>
              <p className="text-xs text-muted-foreground">
                {t('规范化 Claude 请求格式')}
              </p>
            </div>
            <Switch
              checked={localSettings['rtk_setting.enable_claude_normalization']}
              onCheckedChange={(checked) => updateLocal('rtk_setting.enable_claude_normalization', checked)}
              disabled={isSaving}
            />
          </div>

          <div className="flex items-center justify-between">
            <div className="space-y-0.5">
              <Label>{t('远程图片获取')}</Label>
              <p className="text-xs text-muted-foreground">
                {t('启用远程图片 URL 获取')}
              </p>
            </div>
            <Switch
              checked={localSettings['rtk_setting.enable_remote_image_fetch']}
              onCheckedChange={(checked) => updateLocal('rtk_setting.enable_remote_image_fetch', checked)}
              disabled={isSaving}
            />
          </div>
        </div>

        <Separator />

        {/* Save Button */}
        <div className="flex justify-end pt-2">
          <Button
            onClick={handleSaveAll}
            disabled={isSaving || !hasChanges}
            size="lg"
          >
            {isSaving ? t('保存中...') : t('保存设置')}
          </Button>
        </div>
      </CardContent>
    </Card>
  )
}
