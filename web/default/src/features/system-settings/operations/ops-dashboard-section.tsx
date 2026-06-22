import { useState, useEffect, useRef, useCallback } from 'react'
import { useTranslation } from 'react-i18next'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { Label } from '@/components/ui/label'
import { Badge } from '@/components/ui/badge'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { api } from '@/lib/api'

export function OpsDashboardSection() {
  const { t } = useTranslation()
  const [logs, setLogs] = useState<string[]>([])
  const [alerts, setAlerts] = useState<any[]>([])
  const [trends, setTrends] = useState<any[]>([])
  const logEndRef = useRef<HTMLDivElement>(null)
  const abortRef = useRef<AbortController | null>(null)

  useEffect(() => {
    loadLogs()
    loadAlerts()
    loadTrends()
    startLogStream()
    return () => { abortRef.current?.abort() }
  }, [])

  useEffect(() => {
    logEndRef.current?.scrollIntoView({ behavior: 'smooth' })
  }, [logs])

  const loadLogs = async () => {
    try {
      // Fixed: correct path for console logs
      const res = await api.get('/api/user/console-logs')
      if (res.data?.success) setLogs(res.data.data || [])
    } catch {}
  }

  const startLogStream = () => {
    // Use fetch + ReadableStream instead of EventSource to support auth
    const abort = new AbortController()
    abortRef.current = abort

    const stream = async () => {
      try {
        const res = await fetch('/api/user/console-logs/stream', {
          signal: abort.signal,
          credentials: 'include',
        })
        if (!res.ok || !res.body) return
        const reader = res.body.getReader()
        const decoder = new TextDecoder()
        let buffer = ''
        while (true) {
          const { done, value } = await reader.read()
          if (done) break
          buffer += decoder.decode(value, { stream: true })
          const lines = buffer.split('\n')
          buffer = lines.pop() || ''
          for (const line of lines) {
            if (line.startsWith('data: ')) {
              const data = line.slice(6)
              setLogs(prev => [...prev.slice(-499), data])
            }
          }
        }
      } catch {
        // Stream closed or error - retry after 5s
        if (!abort.signal.aborted) {
          setTimeout(stream, 5000)
        }
      }
    }
    stream()
  }

  const loadAlerts = async () => {
    try {
      const res = await api.get('/api/ops_alerts/?limit=20')
      if (res.data?.success) setAlerts(res.data.data || [])
    } catch {}
  }

  const loadTrends = useCallback(async () => {
    try {
      // Fixed: use correct trends endpoint
      const res = await api.get('/api/user/ops/trends')
      if (res.data?.success) {
        const data = res.data.data || []
        // Normalize trend data
        const points = Array.isArray(data) ? data.map((d: any) => ({
          timestamp: d.timestamp || d.time || Math.floor(Date.now() / 1000),
          requests: d.requests || d.total_requests || 0,
          errors: d.errors || d.total_errors || 0,
          avg_latency_ms: d.avg_latency_ms || d.latency || 0,
          error_rate: d.error_rate || 0,
          model: d.model || '',
        })) : []
        setTrends(points)
      }
    } catch {}
  }, [])

  const severityColor = (s: string) => {
    if (s === 'critical') return 'bg-red-100 text-red-800'
    if (s === 'warning') return 'bg-yellow-100 text-yellow-800'
    return 'bg-blue-100 text-blue-800'
  }

  const totalRequests = trends.reduce((s: number, tp: any) => s + (tp.requests || 0), 0)
  const totalErrors = trends.reduce((s: number, tp: any) => s + (tp.errors || 0), 0)
  const avgLatency = trends.length > 0 ? trends.reduce((s: number, tp: any) => s + (tp.avg_latency_ms || 0), 0) / trends.length : 0

  return (
    <Card>
      <CardHeader>
        <CardTitle>{t('Ops Dashboard')}</CardTitle>
        <CardDescription>{t('Real-time Logs / Alerts / Traffic Trends')}</CardDescription>
      </CardHeader>
      <CardContent>
        <Tabs defaultValue="logs" className="w-full">
          <TabsList className="grid w-full grid-cols-3">
            <TabsTrigger value="logs">{t('Real-time Logs')}</TabsTrigger>
            <TabsTrigger value="alerts">{t('Alerts')} ({alerts.length})</TabsTrigger>
            <TabsTrigger value="trends">{t('Trends')}</TabsTrigger>
          </TabsList>

          <TabsContent value="logs" className="space-y-2">
            <div className="flex justify-between items-center">
              <Label className="text-xs text-muted-foreground">{t('Real-time Gateway Logs (SSE)')}</Label>
              <Button variant="outline" size="sm" onClick={loadLogs}>{t('Refresh Buffer')}</Button>
            </div>
            <div className="h-80 overflow-y-auto bg-black text-green-400 rounded p-3 font-mono text-xs space-y-0.5">
              {logs.length === 0 ? (
                <div className="text-gray-500">{t('Waiting for logs...')}</div>
              ) : (
                logs.map((line, i) => (
                  <div key={i} className="whitespace-pre-wrap break-all">{line}</div>
                ))
              )}
              <div ref={logEndRef} />
            </div>
          </TabsContent>

          <TabsContent value="alerts" className="space-y-2">
            <div className="flex justify-between items-center">
              <Label className="text-xs text-muted-foreground">{t('Recent Alert Records')}</Label>
              <Button variant="outline" size="sm" onClick={loadAlerts}>{t('Refresh')}</Button>
            </div>
            {alerts.length === 0 ? (
              <p className="text-sm text-muted-foreground">{t('No alerts')}</p>
            ) : (
              <div className="space-y-2 max-h-80 overflow-y-auto">
                {alerts.map((a: any) => (
                  <div key={a.id} className="flex items-start gap-3 p-3 border rounded-lg">
                    <span className={`text-xs px-2 py-0.5 rounded ${severityColor(a.severity)}`}>{a.severity}</span>
                    <div className="flex-1 min-w-0">
                      <p className="text-sm truncate">{a.message}</p>
                      <p className="text-xs text-muted-foreground">
                        {a.type} · {t('Channel')} #{a.channel_id} · {t('Value')}: {a.value} / {t('Threshold')}: {a.threshold}
                        {a.resolved && <span className="ml-2 text-green-600">✓ {t('Recovered')}</span>}
                      </p>
                    </div>
                  </div>
                ))}
              </div>
            )}
          </TabsContent>

          <TabsContent value="trends" className="space-y-4">
            <div className="grid grid-cols-3 gap-4">
              <div className="p-3 border rounded-lg text-center">
                <div className="text-2xl font-bold">{totalRequests}</div>
                <div className="text-xs text-muted-foreground">{t('Total Requests (2h)')}</div>
              </div>
              <div className="p-3 border rounded-lg text-center">
                <div className="text-2xl font-bold text-red-600">{totalErrors}</div>
                <div className="text-xs text-muted-foreground">{t('Total Errors')}</div>
              </div>
              <div className="p-3 border rounded-lg text-center">
                <div className="text-2xl font-bold">{Math.round(avgLatency)}ms</div>
                <div className="text-xs text-muted-foreground">{t('Average Latency')}</div>
              </div>
            </div>
            <div className="flex justify-between items-center">
              <Label className="text-xs text-muted-foreground">{t('Per-minute Traffic Data (Last 2 Hours)')}</Label>
              <Button variant="outline" size="sm" onClick={loadTrends}>{t('Refresh')}</Button>
            </div>
            {trends.length === 0 ? (
              <p className="text-sm text-muted-foreground">{t('No trend data yet')}</p>
            ) : (
              <div className="h-48 overflow-y-auto space-y-1">
                {trends.slice(-30).reverse().map((tp: any, i: number) => (
                  <div key={i} className="flex items-center gap-3 text-xs font-mono p-1">
                    <span className="text-muted-foreground w-16">{new Date(tp.timestamp * 1000).toLocaleTimeString()}</span>
                    <span className="w-12">req:{tp.requests}</span>
                    <span className={`w-12 ${tp.errors > 0 ? 'text-red-600' : ''}`}>err:{tp.errors}</span>
                    <span className="w-20 truncate">{tp.model || '-'}</span>
                    <span className="w-16">{Math.round(tp.avg_latency_ms)}ms</span>
                    <span className="w-20 text-muted-foreground">{(tp.error_rate * 100).toFixed(1)}%</span>
                    <div className="flex-1 bg-gray-100 rounded-full h-2">
                      <div
                        className={`h-2 rounded-full ${tp.error_rate > 0.1 ? 'bg-red-400' : tp.error_rate > 0.05 ? 'bg-yellow-400' : 'bg-green-400'}`}
                        style={{ width: `${Math.min(tp.error_rate * 100 * 5, 100)}%` }}
                      />
                    </div>
                  </div>
                ))}
              </div>
            )}
          </TabsContent>
        </Tabs>
      </CardContent>
    </Card>
  )
}
