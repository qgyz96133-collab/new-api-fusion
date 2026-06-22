import { useState, useEffect } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Switch } from '@/components/ui/switch'
import { Textarea } from '@/components/ui/textarea'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { api } from '@/lib/api'

export function IntegrationsSection() {
  const { t } = useTranslation()

  // DingTalk
  const [dingtalk, setDingtalk] = useState({ enabled: false, client_id: '', client_secret: '', corp_id: '', redirect_uri: '' })
  // WeCom
  const [wecom, setWecom] = useState({ enabled: false, corp_id: '', agent_id: '', corp_secret: '', redirect_uri: '' })
  // Balance Notify
  const [balanceNotify, setBalanceNotify] = useState({
    enabled: false, threshold: 1.0, threshold_pct: 90, notify_method: 'internal', webhook_url: '', cooldown_hours: 24,
  })
  // Grok Import
  const [grokTokens, setGrokTokens] = useState('')
  const [grokResult, setGrokResult] = useState<any>(null)

  useEffect(() => {
    loadDingtalk()
    loadWecom()
    loadBalanceNotify()
  }, [])

  const loadDingtalk = async () => {
    try {
      // Fixed: correct API paths
      const res = await api.get('/api/user/dingtalk-config')
      if (res.data?.success) setDingtalk(prev => ({ ...prev, ...res.data.data }))
    } catch {}
  }

  const loadWecom = async () => {
    try {
      const res = await api.get('/api/user/wecom-config')
      if (res.data?.success) setWecom(prev => ({ ...prev, ...res.data.data }))
    } catch {}
  }

  const loadBalanceNotify = async () => {
    try {
      const res = await api.get('/api/user/balance-notify')
      if (res.data?.success) setBalanceNotify(prev => ({ ...prev, ...res.data.data }))
    } catch {}
  }

  const saveDingtalk = async () => {
    const res = await api.put('/api/user/dingtalk-config', dingtalk)
    if (res.data?.success) toast.success(t('DingTalk OAuth config saved'))
    else toast.error(res.data?.message || t('Save failed'))
  }

  const saveWecom = async () => {
    const res = await api.put('/api/user/wecom-config', wecom)
    if (res.data?.success) toast.success(t('WeCom OAuth config saved'))
    else toast.error(res.data?.message || t('Save failed'))
  }

  const saveBalanceNotify = async () => {
    const res = await api.put('/api/user/balance-notify', balanceNotify)
    if (res.data?.success) toast.success(t('Balance notify config saved'))
    else toast.error(res.data?.message || t('Save failed'))
  }

  const importGrok = async () => {
    if (!grokTokens.trim()) { toast.error(t('Please enter Token')); return }
    const tokens = grokTokens.split('\n').map(t => t.trim()).filter(Boolean)
    try {
      const res = await api.post('/api/user/grok-import', { tokens, skip_existing: true })
      if (res.data?.success) {
        setGrokResult(res.data.data)
        toast.success(t('Imported {success}/{total}', { success: res.data.data.success, total: res.data.data.total }))
        setGrokTokens('')
      }
    } catch {
      toast.error(t('Import failed'))
    }
  }

  return (
    <Card>
      <CardHeader>
        <CardTitle>{t('Integration Config')}</CardTitle>
        <CardDescription>{t('DingTalk / WeCom OAuth / Balance Notify / Grok Import')}</CardDescription>
      </CardHeader>
      <CardContent>
        <Tabs defaultValue="dingtalk" className="w-full">
          <TabsList className="grid w-full grid-cols-4">
            <TabsTrigger value="dingtalk">{t('DingTalk OAuth')}</TabsTrigger>
            <TabsTrigger value="wecom">{t('WeCom')}</TabsTrigger>
            <TabsTrigger value="balance">{t('Balance Notify')}</TabsTrigger>
            <TabsTrigger value="grok">{t('Grok Import')}</TabsTrigger>
          </TabsList>

          <TabsContent value="dingtalk" className="space-y-4">
            <div className="flex items-center justify-between">
              <Label>{t('Enable DingTalk Login')}</Label>
              <Switch checked={dingtalk.enabled} onCheckedChange={(v) => setDingtalk({ ...dingtalk, enabled: v })} />
            </div>
            <div className="grid grid-cols-2 gap-4">
              <div className="space-y-1">
                <Label className="text-xs">Client ID</Label>
                <Input value={dingtalk.client_id} onChange={(e) => setDingtalk({ ...dingtalk, client_id: e.target.value })} />
              </div>
              <div className="space-y-1">
                <Label className="text-xs">Client Secret</Label>
                <Input type="password" value={dingtalk.client_secret} onChange={(e) => setDingtalk({ ...dingtalk, client_secret: e.target.value })} />
              </div>
              <div className="space-y-1">
                <Label className="text-xs">Corp ID</Label>
                <Input value={dingtalk.corp_id} onChange={(e) => setDingtalk({ ...dingtalk, corp_id: e.target.value })} />
              </div>
              <div className="space-y-1">
                <Label className="text-xs">Redirect URI</Label>
                <Input value={dingtalk.redirect_uri} onChange={(e) => setDingtalk({ ...dingtalk, redirect_uri: e.target.value })} />
              </div>
            </div>
            <Button onClick={saveDingtalk}>{t('Save')}</Button>
          </TabsContent>

          <TabsContent value="wecom" className="space-y-4">
            <div className="flex items-center justify-between">
              <Label>{t('Enable WeCom Login')}</Label>
              <Switch checked={wecom.enabled} onCheckedChange={(v) => setWecom({ ...wecom, enabled: v })} />
            </div>
            <div className="grid grid-cols-2 gap-4">
              <div className="space-y-1">
                <Label className="text-xs">Corp ID</Label>
                <Input value={wecom.corp_id} onChange={(e) => setWecom({ ...wecom, corp_id: e.target.value })} />
              </div>
              <div className="space-y-1">
                <Label className="text-xs">Agent ID</Label>
                <Input value={wecom.agent_id} onChange={(e) => setWecom({ ...wecom, agent_id: e.target.value })} />
              </div>
              <div className="space-y-1">
                <Label className="text-xs">Corp Secret</Label>
                <Input type="password" value={wecom.corp_secret} onChange={(e) => setWecom({ ...wecom, corp_secret: e.target.value })} />
              </div>
              <div className="space-y-1">
                <Label className="text-xs">Redirect URI</Label>
                <Input value={wecom.redirect_uri} onChange={(e) => setWecom({ ...wecom, redirect_uri: e.target.value })} />
              </div>
            </div>
            <Button onClick={saveWecom}>{t('Save')}</Button>
          </TabsContent>

          <TabsContent value="balance" className="space-y-4">
            <div className="flex items-center justify-between">
              <Label>{t('Enable Balance Notification')}</Label>
              <Switch checked={balanceNotify.enabled} onCheckedChange={(v) => setBalanceNotify({ ...balanceNotify, enabled: v })} />
            </div>
            <div className="grid grid-cols-2 gap-4">
              <div className="space-y-1">
                <Label className="text-xs">{t('Balance Threshold')} (USD)</Label>
                <Input type="number" step="0.1" value={balanceNotify.threshold} onChange={(e) => setBalanceNotify({ ...balanceNotify, threshold: parseFloat(e.target.value) || 0 })} />
              </div>
              <div className="space-y-1">
                <Label className="text-xs">{t('Usage Percentage Threshold')} (%)</Label>
                <Input type="number" value={balanceNotify.threshold_pct} onChange={(e) => setBalanceNotify({ ...balanceNotify, threshold_pct: parseInt(e.target.value) || 0 })} />
              </div>
              <div className="space-y-1">
                <Label className="text-xs">{t('Notification Method')}</Label>
                <Select value={balanceNotify.notify_method} onValueChange={(v) => setBalanceNotify({ ...balanceNotify, notify_method: v })}>
                  <SelectTrigger className="w-full"><SelectValue /></SelectTrigger>
                  <SelectContent>
                    <SelectItem value="internal">{t('In-app Notification')}</SelectItem>
                    <SelectItem value="webhook">Webhook</SelectItem>
                  </SelectContent>
                </Select>
              </div>
              <div className="space-y-1">
                <Label className="text-xs">{t('Cooldown Time')} ({t('hours')})</Label>
                <Input type="number" value={balanceNotify.cooldown_hours} onChange={(e) => setBalanceNotify({ ...balanceNotify, cooldown_hours: parseInt(e.target.value) || 0 })} />
              </div>
            </div>
            {balanceNotify.notify_method === 'webhook' && (
              <div className="space-y-1">
                <Label className="text-xs">Webhook URL</Label>
                <Input value={balanceNotify.webhook_url} onChange={(e) => setBalanceNotify({ ...balanceNotify, webhook_url: e.target.value })} placeholder="https://..." />
              </div>
            )}
            <Button onClick={saveBalanceNotify}>{t('Save')}</Button>
          </TabsContent>

          <TabsContent value="grok" className="space-y-4">
            <div className="space-y-2">
              <Label>{t('Batch Import Grok SSO Token')}</Label>
              <p className="text-xs text-muted-foreground">{t('One token per line, automatically creates grok type Channel')}</p>
            </div>
            <Textarea
              value={grokTokens}
              onChange={(e) => setGrokTokens(e.target.value)}
              rows={8}
              className="font-mono text-xs"
              placeholder="sso_token_1&#10;sso_token_2&#10;sso_token_3"
            />
            <Button onClick={importGrok}>{t('Import')}</Button>
            {grokResult && (
              <div className="p-3 border rounded text-sm">
                <p>{t('Total')}: {grokResult.total} · {t('Success')}: <span className="text-green-600">{grokResult.success}</span> · {t('Failed')}: <span className="text-red-600">{grokResult.failed}</span></p>
              </div>
            )}
          </TabsContent>
        </Tabs>
      </CardContent>
    </Card>
  )
}
