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
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { api } from '@/lib/api'

interface Rule { id: number; name: string; enabled: boolean; priority: number; error_codes: string; keywords: string; match_mode: string; platforms: string; passthrough_code: boolean; passthrough_body: boolean; custom_message: string | null; description: string | null }
const API_BASE = '/api/error-passthrough'

export function ErrorPassthroughSection() {
  const { t } = useTranslation()
  const [rules, setRules] = useState<Rule[]>([])
  const [showDialog, setShowDialog] = useState(false)
  const [editing, setEditing] = useState<Rule | null>(null)
  const [loading, setLoading] = useState(true)

  const fetchRules = useCallback(async () => {
    try { const res = await api.get(API_BASE + '/'); if (res.data?.success) setRules(res.data.data || []) } catch {}
    finally { setLoading(false) }
  }, [])
  useEffect(() => { fetchRules() }, [fetchRules])

  const saveRule = async (r: Partial<Rule>) => {
    try {
      const method = r.id ? 'put' : 'post'
      const res = await api[method](API_BASE + '/', r)
      if (res.data?.success) { toast.success(t(r.id ? 'Rule updated' : 'Rule created')); setShowDialog(false); setEditing(null); fetchRules() }
      else toast.error(res.data?.message || t('Save failed'))
    } catch { toast.error(t('Save failed')) }
  }

  const deleteRule = async (id: number) => {
    if (!confirm(t('Delete this rule?'))) return
    try { await api.delete(`${API_BASE}/${id}`); toast.success(t('Deleted')); fetchRules() } catch {}
  }

  return (
    <SettingsSection title={t('Error Passthrough Rules')} description={t('Control how upstream errors are returned to clients')}>
      <div className="flex justify-between items-center mb-4">
        <p className="text-sm text-muted-foreground">{rules.length} {t('rule(s)')}</p>
        <Button onClick={() => { setEditing(null); setShowDialog(true) }} size="sm">+ {t('Add Rule')}</Button>
      </div>
      {loading ? (
        <div className="text-center py-8 text-muted-foreground">{t('Loading...')}</div>
      ) : (
        <Table>
          <TableHeader><TableRow>
            <TableHead>{t('Name')}</TableHead><TableHead>{t('Priority')}</TableHead><TableHead>{t('Error Codes')}</TableHead>
            <TableHead>{t('Keywords')}</TableHead><TableHead>{t('Mode')}</TableHead><TableHead>{t('Status')}</TableHead><TableHead>{t('Actions')}</TableHead>
          </TableRow></TableHeader>
          <TableBody>
            {rules.map(r => (
              <TableRow key={r.id}>
                <TableCell className="font-medium">{r.name}</TableCell>
                <TableCell>{r.priority}</TableCell>
                <TableCell>{r.error_codes}</TableCell>
                <TableCell className="max-w-[200px] truncate">{r.keywords}</TableCell>
                <TableCell><Badge variant="outline">{r.match_mode}</Badge></TableCell>
                <TableCell><Switch checked={r.enabled} onCheckedChange={() => saveRule({ ...r, enabled: !r.enabled })} /></TableCell>
                <TableCell className="space-x-1">
                  <Button variant="outline" size="sm" onClick={() => { setEditing(r); setShowDialog(true) }}>{t('Edit')}</Button>
                  <Button variant="destructive" size="sm" onClick={() => deleteRule(r.id)}>{t('Delete')}</Button>
                </TableCell>
              </TableRow>
            ))}
            {rules.length === 0 && <TableRow><TableCell colSpan={7} className="text-center text-muted-foreground">{t('No rules configured')}</TableCell></TableRow>}
          </TableBody>
        </Table>
      )}
      <Dialog open={showDialog} onOpenChange={setShowDialog}>
        <DialogContent className="max-w-lg">
          <DialogHeader><DialogTitle>{editing ? t('Edit Rule') : t('Add Rule')}</DialogTitle></DialogHeader>
          <RuleForm initial={editing} onSave={saveRule} onCancel={() => { setShowDialog(false); setEditing(null) }} />
        </DialogContent>
      </Dialog>
    </SettingsSection>
  )
}

function RuleForm({ initial, onSave, onCancel }: { initial: Rule | null; onSave: (r: Partial<Rule>) => void; onCancel: () => void }) {
  const { t } = useTranslation()
  const [name, setName] = useState(initial?.name ?? '')
  const [priority, setPriority] = useState(initial?.priority?.toString() ?? '0')
  const [errorCodes, setErrorCodes] = useState(initial?.error_codes ?? '')
  const [keywords, setKeywords] = useState(initial?.keywords ?? '')
  const [matchMode, setMatchMode] = useState(initial?.match_mode ?? 'any')
  const [passthroughCode, setPassthroughCode] = useState(initial?.passthrough_code ?? true)
  const [passthroughBody, setPassthroughBody] = useState(initial?.passthrough_body ?? true)
  const [customMsg, setCustomMsg] = useState(initial?.custom_message ?? '')

  const handleSave = () => {
    // Validate JSON fields
    try {
      if (errorCodes.trim()) JSON.parse(errorCodes)
    } catch {
      toast.error(t('Error Codes must be valid JSON array, e.g. [429, 503]'))
      return
    }
    try {
      if (keywords.trim()) JSON.parse(keywords)
    } catch {
      toast.error(t('Keywords must be valid JSON array, e.g. ["rate limit"]'))
      return
    }
    onSave({
      ...(initial ? { id: initial.id } : {}),
      name, priority: parseInt(priority) || 0,
      error_codes: errorCodes, keywords: keywords,
      match_mode: matchMode,
      passthrough_code: passthroughCode, passthrough_body: passthroughBody,
      custom_message: customMsg || null, enabled: true,
    })
  }

  return (
    <div className="space-y-4">
      <div><Label>{t('Name')}</Label><Input value={name} onChange={e => setName(e.target.value)} /></div>
      <div className="grid grid-cols-2 gap-2">
        <div><Label>{t('Priority')}</Label><Input value={priority} onChange={e => setPriority(e.target.value)} /></div>
        <div><Label>{t('Match Mode')}</Label>
          <Select value={matchMode} onValueChange={setMatchMode}>
            <SelectTrigger><SelectValue /></SelectTrigger>
            <SelectContent>
              <SelectItem value="any">{t('Any')} (OR)</SelectItem>
              <SelectItem value="all">{t('All')} (AND)</SelectItem>
            </SelectContent>
          </Select>
        </div>
      </div>
      <div><Label>{t('Error Codes')} (JSON: [429, 503])</Label><Input value={errorCodes} onChange={e => setErrorCodes(e.target.value)} placeholder="[429, 503]" /></div>
      <div><Label>{t('Keywords')} (JSON: [&quot;rate limit&quot;, &quot;overloaded&quot;])</Label><Input value={keywords} onChange={e => setKeywords(e.target.value)} placeholder='["rate limit"]' /></div>
      <div className="flex items-center gap-4">
        <div className="flex items-center gap-2"><Switch checked={passthroughCode} onCheckedChange={setPassthroughCode} /><Label>{t('Passthrough status code')}</Label></div>
        <div className="flex items-center gap-2"><Switch checked={passthroughBody} onCheckedChange={setPassthroughBody} /><Label>{t('Passthrough error body')}</Label></div>
      </div>
      {!passthroughBody && <div><Label>{t('Custom Message')}</Label><Textarea value={customMsg} onChange={e => setCustomMsg(e.target.value)} /></div>}
      <DialogFooter><Button variant="outline" onClick={onCancel}>{t('Cancel')}</Button><Button onClick={handleSave}>{t('Save')}</Button></DialogFooter>
    </div>
  )
}
