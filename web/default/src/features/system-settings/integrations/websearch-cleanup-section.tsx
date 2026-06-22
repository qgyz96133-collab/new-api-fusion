import { useCallback, useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { SettingsSection } from '../components/settings-section'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Badge } from '@/components/ui/badge'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogFooter } from '@/components/ui/dialog'
import { Label } from '@/components/ui/label'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { api } from '@/lib/api'

interface WebSearchProvider { id: number; type: string; name: string; api_key: string; base_url: string; quota_limit: number; quota_used: number; status: string }
interface CleanupTask { id: number; status: string; filters: string; deleted_rows: number; created_at: string; finished_at: string | null; error_msg: string | null }

function maskApiKey(key: string): string {
  if (!key || key.length <= 8) return '****'
  return key.slice(0, 4) + '****' + key.slice(-4)
}

export function WebSearchSection() {
  const { t } = useTranslation()
  const [providers, setProviders] = useState<WebSearchProvider[]>([])
  const [showDialog, setShowDialog] = useState(false)
  const [newType, setNewType] = useState('brave')
  const [newKey, setNewKey] = useState('')
  const [newUrl, setNewUrl] = useState('')
  const [loading, setLoading] = useState(true)

  const fetchProviders = useCallback(async () => {
    try { const res = await api.get('/api/websearch/'); if (res.data?.success) setProviders(res.data.data || []) } catch {}
    finally { setLoading(false) }
  }, [])
  useEffect(() => { fetchProviders() }, [fetchProviders])

  const addProvider = async () => {
    try {
      const res = await api.post('/api/websearch/', { type: newType, name: newType, api_key: newKey, base_url: newUrl, status: 'active' })
      if (res.data?.success) { toast.success(t('Provider added')); setShowDialog(false); setNewKey(''); setNewUrl(''); fetchProviders() }
      else toast.error(res.data?.message || t('Failed'))
    } catch { toast.error(t('Failed')) }
  }

  const deleteProvider = async (id: number) => {
    try { await api.delete(`/api/websearch/${id}`); toast.success(t('Deleted')); fetchProviders() } catch {}
  }

  return (
    <SettingsSection title={t('Web Search Providers')} description={t('Brave, Tavily, SearXNG configuration')}>
      <div className="flex justify-between items-center mb-4">
        <p className="text-sm text-muted-foreground">{providers.length} {t('provider(s)')}</p>
        <Button onClick={() => setShowDialog(true)} size="sm">+ {t('Add Provider')}</Button>
      </div>
      {loading ? (
        <div className="text-center py-8 text-muted-foreground">{t('Loading...')}</div>
      ) : (
        <Table>
          <TableHeader><TableRow>
            <TableHead>{t('Type')}</TableHead><TableHead>{t('API Key')}</TableHead><TableHead>{t('Quota')}</TableHead>
            <TableHead>{t('Status')}</TableHead><TableHead>{t('Actions')}</TableHead>
          </TableRow></TableHeader>
          <TableBody>
            {providers.map(p => (
              <TableRow key={p.id}>
                <TableCell><Badge>{p.type}</Badge></TableCell>
                <TableCell className="font-mono text-xs">{maskApiKey(p.api_key)}</TableCell>
                <TableCell>{p.quota_used}/{p.quota_limit || '∞'}</TableCell>
                <TableCell><Badge variant={p.status === 'active' ? 'default' : 'secondary'}>{p.status}</Badge></TableCell>
                <TableCell><Button variant="destructive" size="sm" onClick={() => deleteProvider(p.id)}>{t('Delete')}</Button></TableCell>
              </TableRow>
            ))}
            {providers.length === 0 && <TableRow><TableCell colSpan={5} className="text-center text-muted-foreground">{t('No providers configured')}</TableCell></TableRow>}
          </TableBody>
        </Table>
      )}
      <Dialog open={showDialog} onOpenChange={setShowDialog}>
        <DialogContent><DialogHeader><DialogTitle>{t('Add Web Search Provider')}</DialogTitle></DialogHeader>
          <div className="space-y-4">
            <div><Label>{t('Type')}</Label>
              <Select value={newType} onValueChange={setNewType}>
                <SelectTrigger className="w-full"><SelectValue /></SelectTrigger>
                <SelectContent>
                  <SelectItem value="brave">Brave</SelectItem>
                  <SelectItem value="tavily">Tavily</SelectItem>
                  <SelectItem value="searxng">SearXNG</SelectItem>
                </SelectContent>
              </Select>
            </div>
            <div><Label>{t('API Key')}</Label><Input value={newKey} onChange={e => setNewKey(e.target.value)} /></div>
            <div><Label>{t('Base URL')} ({t('optional')})</Label><Input value={newUrl} onChange={e => setNewUrl(e.target.value)} placeholder="https://..." /></div>
          </div>
          <DialogFooter><Button variant="outline" onClick={() => setShowDialog(false)}>{t('Cancel')}</Button><Button onClick={addProvider}>{t('Add')}</Button></DialogFooter>
        </DialogContent>
      </Dialog>
    </SettingsSection>
  )
}

export function UsageCleanupSection() {
  const { t } = useTranslation()
  const [tasks, setTasks] = useState<CleanupTask[]>([])
  const [showDialog, setShowDialog] = useState(false)
  const [startDate, setStartDate] = useState('')
  const [endDate, setEndDate] = useState('')
  const [loading, setLoading] = useState(true)

  const fetchTasks = useCallback(async () => {
    try { const res = await api.get('/api/usage-cleanup/'); if (res.data?.success) setTasks(res.data.data || []) } catch {}
    finally { setLoading(false) }
  }, [])
  useEffect(() => { fetchTasks() }, [fetchTasks])

  const createTask = async () => {
    if (!startDate || !endDate) { toast.error(t('Date range required')); return }
    try {
      // Fixed: serialize filters as JSON string if backend expects string
      const res = await api.post('/api/usage-cleanup/', {
        filters: JSON.stringify({ start_time: new Date(startDate).toISOString(), end_time: new Date(endDate).toISOString() })
      })
      if (res.data?.success) { toast.success(t('Task created')); setShowDialog(false); fetchTasks() }
      else toast.error(res.data?.message || t('Failed'))
    } catch { toast.error(t('Failed')) }
  }

  const cancelTask = async (id: number) => {
    try { await api.post(`/api/usage-cleanup/${id}/cancel`); toast.success(t('Canceled')); fetchTasks() } catch {}
  }

  const statusVariant = (s: string): any => ({ succeeded: 'default', failed: 'destructive', running: 'outline', pending: 'secondary', canceled: 'secondary' }[s] || 'secondary')

  return (
    <SettingsSection title={t('Usage Log Cleanup')} description={t('Schedule deletion of old usage logs')}>
      <div className="flex justify-between items-center mb-4">
        <p className="text-sm text-muted-foreground">{tasks.length} {t('task(s)')}</p>
        <Button onClick={() => setShowDialog(true)} size="sm">+ {t('New Cleanup Task')}</Button>
      </div>
      {loading ? (
        <div className="text-center py-8 text-muted-foreground">{t('Loading...')}</div>
      ) : (
        <Table>
          <TableHeader><TableRow>
            <TableHead>ID</TableHead><TableHead>{t('Status')}</TableHead><TableHead>{t('Deleted Rows')}</TableHead>
            <TableHead>{t('Created')}</TableHead><TableHead>{t('Finished')}</TableHead><TableHead>{t('Actions')}</TableHead>
          </TableRow></TableHeader>
          <TableBody>
            {tasks.map(tk => (
              <TableRow key={tk.id}>
                <TableCell>#{tk.id}</TableCell>
                <TableCell><Badge variant={statusVariant(tk.status)}>{tk.status}</Badge></TableCell>
                <TableCell>{tk.deleted_rows}</TableCell>
                <TableCell>{new Date(tk.created_at).toLocaleString()}</TableCell>
                <TableCell>{tk.finished_at ? new Date(tk.finished_at).toLocaleString() : '-'}</TableCell>
                <TableCell>{(tk.status === 'pending' || tk.status === 'running') && <Button variant="outline" size="sm" onClick={() => cancelTask(tk.id)}>{t('Cancel')}</Button>}</TableCell>
              </TableRow>
            ))}
            {tasks.length === 0 && <TableRow><TableCell colSpan={6} className="text-center text-muted-foreground">{t('No tasks')}</TableCell></TableRow>}
          </TableBody>
        </Table>
      )}
      <Dialog open={showDialog} onOpenChange={setShowDialog}>
        <DialogContent><DialogHeader><DialogTitle>{t('New Cleanup Task')}</DialogTitle></DialogHeader>
          <div className="space-y-4">
            <div><Label>{t('Start Date')}</Label><Input type="datetime-local" value={startDate} onChange={e => setStartDate(e.target.value)} /></div>
            <div><Label>{t('End Date')}</Label><Input type="datetime-local" value={endDate} onChange={e => setEndDate(e.target.value)} /></div>
            <p className="text-xs text-muted-foreground">⚠️ {t('This will permanently delete usage logs in the selected date range. Max range: 90 days.')}</p>
          </div>
          <DialogFooter><Button variant="outline" onClick={() => setShowDialog(false)}>{t('Cancel')}</Button><Button variant="destructive" onClick={createTask}>{t('Create Task')}</Button></DialogFooter>
        </DialogContent>
      </Dialog>
    </SettingsSection>
  )
}
