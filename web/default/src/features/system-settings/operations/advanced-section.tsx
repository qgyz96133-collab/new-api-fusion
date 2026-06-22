import { useState, useEffect, useCallback } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Switch } from '@/components/ui/switch'
import { Separator } from '@/components/ui/separator'
import { Textarea } from '@/components/ui/textarea'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { updateSystemOption } from '@/features/system-settings/api'
import { api } from '@/lib/api'

export function AdvancedSection() {
  const { t } = useTranslation()

  // Fallback chain state
  const [fallbackConfig, setFallbackConfig] = useState<any>({ provider_chains: {}, model_mappings: {} })
  const [fallbackJson, setFallbackJson] = useState('')

  // Channel scores
  const [scores, setScores] = useState<any[]>([])

  // uTLS state
  const [utlsEnabled, setUtlsEnabled] = useState(false)
  const [utlsFingerprint, setUtlsFingerprint] = useState('chrome')
  const [utlsSaving, setUtlsSaving] = useState(false)

  // Prompt replacement state
  const [promptRules, setPromptRules] = useState<{ old: string; new: string }[]>([])
  const [promptMode, setPromptMode] = useState('overwrite')

  // Alert thresholds
  const [alertThresholds, setAlertThresholds] = useState({
    error_rate_warn: 0.05, error_rate_crit: 0.2,
    latency_warn_ms: 5000, latency_crit_ms: 30000, channel_down_min: 5,
  })

  // Loading state
  const [loading, setLoading] = useState(true)

  // Load data on mount
  useEffect(() => {
    Promise.all([
      loadFallbackConfig(),
      loadScores(),
      loadUtlsStatus(),
      loadPromptRules(),
    ]).finally(() => setLoading(false))
  }, [])

  const loadFallbackConfig = async () => {
    try {
      const res = await api.get('/api/user/fallback-config')
      if (res.data?.success) {
        setFallbackConfig(res.data.data)
        setFallbackJson(JSON.stringify(res.data.data, null, 2))
      }
    } catch {}
  }

  const loadScores = async () => {
    try {
      const res = await api.get('/api/user/channel-scores')
      if (res.data?.success) setScores(res.data.data || [])
    } catch {}
  }

  const loadUtlsStatus = async () => {
    try {
      const res = await api.get('/api/option/', { disableDuplicate: true })
      if (res.data?.success) {
        // Options come as an array of {key, value} objects
        const options = res.data.data
        const optionMap: Record<string, string> = {}
        if (Array.isArray(options)) {
          options.forEach((o: any) => { optionMap[o.key] = o.value })
        } else if (typeof options === 'object') {
          Object.assign(optionMap, options)
        }
        if (optionMap['utls_enabled'] === 'true') setUtlsEnabled(true)
        if (optionMap['utls_fingerprint']) setUtlsFingerprint(optionMap['utls_fingerprint'])
      }
    } catch {}
  }

  const loadPromptRules = async () => {
    try {
      const res = await api.get('/api/user/prompt-replacements')
      if (res.data?.success) {
        setPromptRules(res.data.data?.rules || [])
        setPromptMode(res.data.data?.mode || 'overwrite')
      }
    } catch {}
  }

  const saveFallback = async () => {
    try {
      const parsed = JSON.parse(fallbackJson)
      const res = await api.put('/api/user/fallback-config', parsed)
      if (res.data?.success) {
        toast.success(t('Fallback config saved'))
        setFallbackConfig(parsed)
      } else {
        toast.error(res.data?.message || t('Save failed'))
      }
    } catch (e: any) {
      toast.error(t('JSON format error') + ': ' + e.message)
    }
  }

  const saveUtlsConfig = useCallback(async () => {
    setUtlsSaving(true)
    try {
      // Save as two separate key-value updates
      const r1 = await updateSystemOption({ key: 'utls_fingerprint', value: utlsFingerprint })
      const r2 = await updateSystemOption({ key: 'utls_enabled', value: utlsEnabled ? 'true' : 'false' })
      if (r1.success && r2.success) {
        toast.success(utlsEnabled ? t('uTLS enabled ({fp})', { fp: utlsFingerprint }) : t('uTLS disabled'))
      } else {
        toast.error(r1.message || r2.message || t('Save failed'))
      }
    } catch {
      toast.error(t('Save failed'))
    } finally {
      setUtlsSaving(false)
    }
  }, [utlsEnabled, utlsFingerprint, t])

  const savePromptRules = async () => {
    try {
      const res = await api.put('/api/user/prompt-replacements', { rules: promptRules, mode: promptMode })
      if (res.data?.success) toast.success(t('Replacement rules saved'))
      else toast.error(res.data?.message || t('Save failed'))
    } catch {
      toast.error(t('Save failed'))
    }
  }

  const saveAlertThresholds = async () => {
    try {
      // Fixed: correct API path
      const res = await api.put('/api/user/ops/alert-thresholds', alertThresholds)
      if (res.data?.success) toast.success(t('Alert thresholds saved'))
      else toast.error(res.data?.message || t('Save failed'))
    } catch {
      toast.error(t('Save failed'))
    }
  }

  const addPromptRule = () => setPromptRules([...promptRules, { old: '', new: '' }])
  const removePromptRule = (i: number) => setPromptRules(promptRules.filter((_, idx) => idx !== i))
  const updatePromptRule = (i: number, field: 'old' | 'new', value: string) => {
    const updated = [...promptRules]
    updated[i] = { ...updated[i], [field]: value }
    setPromptRules(updated)
  }

  return (
    <Card>
      <CardHeader>
        <CardTitle>{t('Advanced Ops')}</CardTitle>
        <CardDescription>{t('Fallback Chain / Channel Scores / uTLS / Prompt Replacement / Alert Thresholds')}</CardDescription>
      </CardHeader>
      <CardContent className="space-y-6">
        <Tabs defaultValue="fallback" className="w-full">
          <TabsList className="grid w-full grid-cols-5">
            <TabsTrigger value="fallback">Fallback</TabsTrigger>
            <TabsTrigger value="scores">{t('Channel Scores')}</TabsTrigger>
            <TabsTrigger value="utls">uTLS</TabsTrigger>
            <TabsTrigger value="prompt">{t('Prompt Replacement')}</TabsTrigger>
            <TabsTrigger value="alerts">{t('Alert Thresholds')}</TabsTrigger>
          </TabsList>

          <TabsContent value="fallback" className="space-y-4">
            <div className="space-y-2">
              <Label>{t('Fallback Chain + Model Mapping (JSON)')}</Label>
              <Textarea
                value={fallbackJson}
                onChange={(e) => setFallbackJson(e.target.value)}
                rows={12}
                className="font-mono text-xs"
                placeholder='{"provider_chains":{"gemini-cli-oauth":["gemini-antigravity"]},"model_mappings":{"gemini-claude-opus-4-5":{"channel_type":14,"model":"claude-opus-4-5","enabled":true}}}'
              />
              <p className="text-xs text-muted-foreground">
                {t('provider_chains: fallback chain when a provider fails. model_mappings: cross-provider model routing.')}
              </p>
            </div>
            <Button onClick={saveFallback}>{t('Save Fallback Config')}</Button>
          </TabsContent>

          <TabsContent value="scores" className="space-y-4">
            <div className="flex justify-between items-center">
              <Label>{t('Channel Real-time Scores')}</Label>
              <Button variant="outline" size="sm" onClick={loadScores}>{t('Refresh')}</Button>
            </div>
            {scores.length === 0 ? (
              <p className="text-sm text-muted-foreground">{t('No score data yet (generated after API requests)')}</p>
            ) : (
              <div className="space-y-2">
                {scores.map((s: any) => (
                  <div key={s.channel_id} className="flex items-center justify-between p-3 border rounded-lg">
                    <div>
                      <span className="font-medium">{t('Channel')} #{s.channel_id}</span>
                      {s.in_cooldown && <span className="ml-2 text-xs bg-red-100 text-red-800 px-2 py-0.5 rounded">{t('Cooldown')} {Math.round(s.cooldown_secs)}s</span>}
                    </div>
                    <div className="flex gap-4 text-sm text-muted-foreground">
                      <span>✅ {s.success_count}</span>
                      <span>❌ {s.error_count}</span>
                      <span>⏱ {Math.round(s.avg_latency_ms)}ms</span>
                      {s.consecutive_429 > 0 && <span className="text-orange-600">429×{s.consecutive_429}</span>}
                    </div>
                  </div>
                ))}
              </div>
            )}
          </TabsContent>

          <TabsContent value="utls" className="space-y-4">
            <div className="flex items-center justify-between">
              <div className="space-y-0.5">
                <Label>{t('uTLS Browser Fingerprint')}</Label>
                <p className="text-sm text-muted-foreground">{t('Use uTLS to simulate browser TLS fingerprint, prevent upstream detection')}</p>
              </div>
              <Switch checked={utlsEnabled} onCheckedChange={setUtlsEnabled} />
            </div>
            <div className="space-y-2">
              <Label>{t('Fingerprint Type')}</Label>
              <Select value={utlsFingerprint} onValueChange={setUtlsFingerprint} disabled={!utlsEnabled}>
                <SelectTrigger className="w-full">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="chrome">Chrome</SelectItem>
                  <SelectItem value="firefox">Firefox</SelectItem>
                  <SelectItem value="safari">Safari</SelectItem>
                  <SelectItem value="edge">Edge</SelectItem>
                </SelectContent>
              </Select>
            </div>
            <Button onClick={saveUtlsConfig} disabled={utlsSaving}>
              {utlsSaving ? t('Saving...') : t('Save Config')}
            </Button>
          </TabsContent>

          <TabsContent value="prompt" className="space-y-4">
            <div className="space-y-2">
              <Label>{t('System Prompt Text Replacement Rules')}</Label>
              <p className="text-xs text-muted-foreground">{t('Automatically replace specified text in system prompts')}</p>
            </div>
            {promptRules.map((rule, i) => (
              <div key={i} className="flex gap-2 items-center">
                <Input value={rule.old} onChange={(e) => updatePromptRule(i, 'old', e.target.value)} placeholder={t('Find text')} className="flex-1" />
                <span>→</span>
                <Input value={rule.new} onChange={(e) => updatePromptRule(i, 'new', e.target.value)} placeholder={t('Replace with')} className="flex-1" />
                <Button variant="ghost" size="sm" onClick={() => removePromptRule(i)}>✕</Button>
              </div>
            ))}
            <div className="flex gap-2">
              <Button variant="outline" onClick={addPromptRule}>+ {t('Add Rule')}</Button>
              <Button onClick={savePromptRules}>{t('Save')}</Button>
            </div>
          </TabsContent>

          <TabsContent value="alerts" className="space-y-4">
            <Label>{t('Channel Alert Thresholds')}</Label>
            <div className="grid grid-cols-2 gap-4">
              <div className="space-y-1">
                <Label className="text-xs">{t('Error Rate Warning')} ({t('e.g. 0.05 = 5%')})</Label>
                <Input type="number" step="0.01" value={alertThresholds.error_rate_warn} onChange={(e) => setAlertThresholds({ ...alertThresholds, error_rate_warn: parseFloat(e.target.value) || 0 })} />
              </div>
              <div className="space-y-1">
                <Label className="text-xs">{t('Error Rate Critical')} ({t('e.g. 0.20 = 20%')})</Label>
                <Input type="number" step="0.01" value={alertThresholds.error_rate_crit} onChange={(e) => setAlertThresholds({ ...alertThresholds, error_rate_crit: parseFloat(e.target.value) || 0 })} />
              </div>
              <div className="space-y-1">
                <Label className="text-xs">{t('Latency Warning')} (ms)</Label>
                <Input type="number" value={alertThresholds.latency_warn_ms} onChange={(e) => setAlertThresholds({ ...alertThresholds, latency_warn_ms: parseInt(e.target.value) || 0 })} />
              </div>
              <div className="space-y-1">
                <Label className="text-xs">{t('Latency Critical')} (ms)</Label>
                <Input type="number" value={alertThresholds.latency_crit_ms} onChange={(e) => setAlertThresholds({ ...alertThresholds, latency_crit_ms: parseInt(e.target.value) || 0 })} />
              </div>
              <div className="space-y-1">
                <Label className="text-xs">{t('Consecutive Errors Offline Threshold')}</Label>
                <Input type="number" value={alertThresholds.channel_down_min} onChange={(e) => setAlertThresholds({ ...alertThresholds, channel_down_min: parseInt(e.target.value) || 0 })} />
              </div>
            </div>
            <Button onClick={saveAlertThresholds}>{t('Save Thresholds')}</Button>
          </TabsContent>
        </Tabs>
      </CardContent>
    </Card>
  )
}
