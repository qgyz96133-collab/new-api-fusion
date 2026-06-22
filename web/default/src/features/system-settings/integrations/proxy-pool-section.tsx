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

interface Proxy { id: number; name: string; protocol: string; host: string; port: number; username: string; password: string; status: string; note: string }
const API_BASE = '/api/proxy'

export function ProxyPoolSection() {
  const { t } = useTranslation()
  const [proxies, setProxies] = useState<Proxy[]>([])
  const [showDialog, setShowDialog] = useState(false)
  const [editing, setEditing] = useState<Proxy | null>(null)
  const [loading, setLoading] = useState(true)

  const fetchProxies = useCallback(async () => {
    try { const res = await api.get(API_BASE + '/'); if (res.data?.success) setProxies(res.data.data || []) } catch {}
    finally { setLoading(false) }
  }, [])
  useEffect(() => { fetchProxies() }, [fetchProxies])

  const saveProxy = async (p: Partial<Proxy>) => {
    try {
      const method = p.id ? 'put' : 'post'
      const res = await api[method](API_BASE + '/', p)
      if (res.data?.success) { toast.success(t(p.id ? 'Proxy updated' : 'Proxy created')); setShowDialog(false); setEditing(null); fetchProxies() }
      else toast.error(res.data?.message || t('Save failed'))
    } catch { toast.error(t('Save failed')) }
  }

  const deleteProxy = async (id: number) => {
    if (!confirm(t('Delete this proxy?'))) return
    try { await api.delete(`${API_BASE}/${id}`); toast.success(t('Deleted')); fetchProxies() } catch {}
  }

  return (
    <SettingsSection title={t('Proxy Pool')} description={t('Outbound proxy management')}>
      <div className="flex justify-between items-center mb-4">
        <p className="text-sm text-muted-foreground">{proxies.length} {t('proxy(ies)')}</p>
        <Button onClick={() => { setEditing(null); setShowDialog(true) }} size="sm">+ {t('Add Proxy')}</Button>
      </div>
      {loading ? (
        <div className="text-center py-8 text-muted-foreground">{t('Loading...')}</div>
      ) : (
        <Table>
          <TableHeader><TableRow>
            <TableHead>{t('Name')}</TableHead><TableHead>{t('Protocol')}</TableHead><TableHead>{t('Host')}:{t('Port')}</TableHead>
            <TableHead>{t('Status')}</TableHead><TableHead>{t('Actions')}</TableHead>
          </TableRow></TableHeader>
          <TableBody>
            {proxies.map(p => (
              <TableRow key={p.id}>
                <TableCell className="font-medium">{p.name}</TableCell>
                <TableCell><Badge variant="outline">{p.protocol}</Badge></TableCell>
                <TableCell>{p.host}:{p.port}</TableCell>
                <TableCell><Badge variant={p.status === 'active' ? 'default' : 'secondary'}>{p.status}</Badge></TableCell>
                <TableCell className="space-x-1">
                  <Button variant="outline" size="sm" onClick={() => { setEditing(p); setShowDialog(true) }}>{t('Edit')}</Button>
                  <Button variant="destructive" size="sm" onClick={() => deleteProxy(p.id)}>{t('Delete')}</Button>
                </TableCell>
              </TableRow>
            ))}
            {proxies.length === 0 && <TableRow><TableCell colSpan={5} className="text-center text-muted-foreground">{t('No proxies configured')}</TableCell></TableRow>}
          </TableBody>
        </Table>
      )}
      <Dialog open={showDialog} onOpenChange={setShowDialog}>
        <DialogContent className="max-w-md">
          <DialogHeader><DialogTitle>{editing ? t('Edit Proxy') : t('Add Proxy')}</DialogTitle></DialogHeader>
          <ProxyForm initial={editing} onSave={saveProxy} onCancel={() => { setShowDialog(false); setEditing(null) }} />
        </DialogContent>
      </Dialog>
    </SettingsSection>
  )
}

function ProxyForm({ initial, onSave, onCancel }: { initial: Proxy | null; onSave: (p: Partial<Proxy>) => void; onCancel: () => void }) {
  const { t } = useTranslation()
  const [name, setName] = useState(initial?.name ?? '')
  const [protocol, setProtocol] = useState(initial?.protocol ?? 'http')
  const [host, setHost] = useState(initial?.host ?? '')
  const [port, setPort] = useState(initial?.port?.toString() ?? '')
  const [username, setUsername] = useState(initial?.username ?? '')
  const [password, setPassword] = useState(initial?.password ?? '')
  return (
    <div className="space-y-4">
      <div><Label>{t('Name')}</Label><Input value={name} onChange={e => setName(e.target.value)} /></div>
      <div><Label>{t('Protocol')}</Label>
        <Select value={protocol} onValueChange={setProtocol}><SelectTrigger><SelectValue /></SelectTrigger>
          <SelectContent><SelectItem value="http">HTTP</SelectItem><SelectItem value="https">HTTPS</SelectItem><SelectItem value="socks5">SOCKS5</SelectItem></SelectContent>
        </Select>
      </div>
      <div className="grid grid-cols-2 gap-2">
        <div><Label>{t('Host')}</Label><Input value={host} onChange={e => setHost(e.target.value)} /></div>
        <div><Label>{t('Port')}</Label><Input value={port} onChange={e => setPort(e.target.value)} placeholder="8080" /></div>
      </div>
      <div className="grid grid-cols-2 gap-2">
        <div><Label>{t('Username')} ({t('optional')})</Label><Input value={username} onChange={e => setUsername(e.target.value)} /></div>
        <div><Label>{t('Password')} ({t('optional')})</Label><Input type="password" value={password} onChange={e => setPassword(e.target.value)} /></div>
      </div>
      <DialogFooter><Button variant="outline" onClick={onCancel}>{t('Cancel')}</Button><Button onClick={() => onSave({ ...(initial ? { id: initial.id } : {}), name, protocol, host, port: parseInt(port) || 0, username, password, status: 'active' })}>{t('Save')}</Button></DialogFooter>
    </div>
  )
}
