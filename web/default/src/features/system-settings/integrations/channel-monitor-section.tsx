import { useCallback, useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { SettingsSection } from '../components/settings-section'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Switch } from '@/components/ui/switch'
import { Badge } from '@/components/ui/badge'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogFooter } from '@/components/ui/dialog'
import { Label } from '@/components/ui/label'
import { Textarea } from '@/components/ui/textarea'
import { api } from '@/lib/api'

interface ChannelMonitor { id: number; name: string; channel_id: number; models: string; enabled: boolean; check_interval: number; last_checked_at: string | null; note: string }
interface ChannelMonitorHistory { id: number; monitor_id: number; model: string; success: boolean; latency_ms: number; status_code: number; message: string; checked_at: string }

const API_BASE = '/api/channel-monitor'

export function ChannelMonitorSection() {
  const { t } = useTranslation()
  const [monitors, setMonitors] = useState<ChannelMonitor[]>([])
  const [showDialog, setShowDialog] = useState(false)
  const [editing, setEditing] = useState<ChannelMonitor | null>(null)
  const [history, setHistory] = useState<ChannelMonitorHistory[]>([])
  const [showHistory, setShowHistory] = useState<number | null>(null)
  const [loading, setLoading] = useState(true)

  const fetchMonitors = useCallback(async () => {
    try {
      const res = await api.get(API_BASE + '/')
      if (res.data?.success) setMonitors(res.data.data || [])
    } catch (e: any) { toast.error(t('Failed to load monitors') + ': ' + (e.message || '')) }
    finally { setLoading(false) }
  }, [t])

  useEffect(() => { fetchMonitors() }, [fetchMonitors])

  const fetchHistory = async (monitorId: number) => {
    try {
      const res = await api.get(`${API_BASE}/${monitorId}/history`, { params: { limit: 50 } })
      if (res.data?.success) setHistory(res.data.data || [])
      setShowHistory(monitorId)
    } catch { toast.error(t('Failed to load history')) }
  }

  const saveMonitor = async (monitor: Partial<ChannelMonitor>) => {
    try {
      const method = monitor.id ? 'put' : 'post'
      const res = await api[method](API_BASE + '/', monitor)
      if (res.data?.success) { toast.success(t(monitor.id ? 'Monitor updated' : 'Monitor created')); setShowDialog(false); setEditing(null); fetchMonitors() }
      else toast.error(res.data?.message || t('Save failed'))
    } catch (e: any) { toast.error(t('Save failed') + ': ' + (e.message || '')) }
  }

  const deleteMonitor = async (id: number) => {
    if (!confirm(t('Delete this monitor?'))) return
    try { await api.delete(`${API_BASE}/${id}`); toast.success(t('Monitor deleted')); fetchMonitors() }
    catch { toast.error(t('Delete failed')) }
  }

  const toggleEnabled = async (monitor: ChannelMonitor) => { await saveMonitor({ ...monitor, enabled: !monitor.enabled }) }

  return (
    <SettingsSection title={t('Channel Monitor')} description={t('Scheduled health checks for channels')}>
      <div className="flex justify-between items-center mb-4">
        <p className="text-sm text-muted-foreground">{monitors.length} {t('monitor(s) configured')}</p>
        <Button onClick={() => { setEditing(null); setShowDialog(true) }} size="sm">+ {t('Add Monitor')}</Button>
      </div>
      {loading ? (
        <div className="text-center py-8 text-muted-foreground">{t('Loading...')}</div>
      ) : (
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>{t('Name')}</TableHead><TableHead>{t('Channel')}</TableHead><TableHead>{t('Interval')}</TableHead>
              <TableHead>{t('Last Check')}</TableHead><TableHead>{t('Status')}</TableHead><TableHead>{t('Actions')}</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {monitors.map((m) => (
              <TableRow key={m.id}>
                <TableCell className="font-medium">{m.name}</TableCell>
                <TableCell>#{m.channel_id}</TableCell>
                <TableCell>{m.check_interval}s</TableCell>
                <TableCell>{m.last_checked_at ? new Date(m.last_checked_at).toLocaleString() : t('Never')}</TableCell>
                <TableCell><Switch checked={m.enabled} onCheckedChange={() => toggleEnabled(m)} /></TableCell>
                <TableCell className="space-x-1">
                  <Button variant="outline" size="sm" onClick={() => fetchHistory(m.id)}>{t('History')}</Button>
                  <Button variant="outline" size="sm" onClick={() => { setEditing(m); setShowDialog(true) }}>{t('Edit')}</Button>
                  <Button variant="destructive" size="sm" onClick={() => deleteMonitor(m.id)}>{t('Delete')}</Button>
                </TableCell>
              </TableRow>
            ))}
            {monitors.length === 0 && <TableRow><TableCell colSpan={6} className="text-center text-muted-foreground">{t('No monitors configured')}</TableCell></TableRow>}
          </TableBody>
        </Table>
      )}
      <Dialog open={showDialog} onOpenChange={setShowDialog}>
        <DialogContent className="max-w-lg">
          <DialogHeader><DialogTitle>{editing ? t('Edit Monitor') : t('Add Monitor')}</DialogTitle></DialogHeader>
          <MonitorForm initial={editing} onSave={saveMonitor} onCancel={() => { setShowDialog(false); setEditing(null) }} />
        </DialogContent>
      </Dialog>
      <Dialog open={showHistory !== null} onOpenChange={() => setShowHistory(null)}>
        <DialogContent className="max-w-2xl max-h-[80vh] overflow-y-auto">
          <DialogHeader><DialogTitle>{t('Check History')}</DialogTitle></DialogHeader>
          <Table>
            <TableHeader><TableRow>
              <TableHead>{t('Model')}</TableHead><TableHead>{t('Result')}</TableHead><TableHead>{t('Latency')}</TableHead>
              <TableHead>{t('Status Code')}</TableHead><TableHead>{t('Time')}</TableHead>
            </TableRow></TableHeader>
            <TableBody>
              {history.map((h) => (
                <TableRow key={h.id}>
                  <TableCell>{h.model}</TableCell>
                  <TableCell><Badge variant={h.success ? 'default' : 'destructive'}>{h.success ? 'OK' : 'FAIL'}</Badge></TableCell>
                  <TableCell>{h.latency_ms}ms</TableCell>
                  <TableCell>{h.status_code}</TableCell>
                  <TableCell>{new Date(h.checked_at).toLocaleString()}</TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </DialogContent>
      </Dialog>
    </SettingsSection>
  )
}

function MonitorForm({ initial, onSave, onCancel }: { initial: ChannelMonitor | null; onSave: (m: Partial<ChannelMonitor>) => void; onCancel: () => void }) {
  const { t } = useTranslation()
  const [name, setName] = useState(initial?.name ?? '')
  const [channelId, setChannelId] = useState(initial?.channel_id?.toString() ?? '')
  const [models, setModels] = useState(() => { if (initial?.models) { try { return JSON.parse(initial.models).join(', ') } catch { return initial.models } } return '' })
  const [interval_, setInterval_] = useState(initial?.check_interval?.toString() ?? '300')
  const [note, setNote] = useState(initial?.note ?? '')

  const handleSubmit = () => {
    const modelArr = models.split(',').map(s => s.trim()).filter(Boolean)
    onSave({ ...(initial ? { id: initial.id } : {}), name, channel_id: parseInt(channelId), models: JSON.stringify(modelArr), check_interval: parseInt(interval_) || 300, enabled: initial?.enabled ?? true, note })
  }

  return (
    <div className="space-y-4">
      <div><Label>{t('Name')}</Label><Input value={name} onChange={e => setName(e.target.value)} placeholder={t('My Monitor')} /></div>
      <div><Label>{t('Channel ID')}</Label><Input value={channelId} onChange={e => setChannelId(e.target.value)} placeholder="1" /></div>
      <div><Label>{t('Models')} ({t('comma-separated')})</Label><Input value={models} onChange={e => setModels(e.target.value)} placeholder="gpt-4o, claude-sonnet-4" /></div>
      <div><Label>{t('Check Interval')} ({t('seconds')})</Label><Input value={interval_} onChange={e => setInterval_(e.target.value)} placeholder="300" /></div>
      <div><Label>{t('Note')}</Label><Textarea value={note} onChange={e => setNote(e.target.value)} placeholder={t('Optional note')} /></div>
      <DialogFooter><Button variant="outline" onClick={onCancel}>{t('Cancel')}</Button><Button onClick={handleSubmit}>{t('Save')}</Button></DialogFooter>
    </div>
  )
}
