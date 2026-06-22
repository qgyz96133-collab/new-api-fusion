import { useState, useEffect } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Textarea } from '@/components/ui/textarea'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { api } from '@/lib/api'

interface ToolInfo { id: string; name: string; envVar: string; icon: string }

export function CliToolsSection() {
  const { t } = useTranslation()
  const [tools, setTools] = useState<ToolInfo[]>([])
  const [selectedTool, setSelectedTool] = useState('claude')
  const [baseUrl, setBaseUrl] = useState(window.location.origin)
  const [apiKey, setApiKey] = useState('')
  const [model, setModel] = useState('')
  const [result, setResult] = useState<any>(null)
  const [loading, setLoading] = useState(true)
  const [generating, setGenerating] = useState(false)

  useEffect(() => {
    loadTools()
  }, [])

  const loadTools = async () => {
    try {
      // Fixed: correct API path
      const res = await api.get('/api/user/tool-configs')
      if (res.data?.success) setTools(res.data.data || [])
    } catch {}
    finally { setLoading(false) }
  }

  const generate = async () => {
    if (!apiKey) { toast.error(t('Please enter API Key')); return }
    setGenerating(true)
    try {
      // Fixed: correct API path
      const res = await api.post('/api/user/tool-configs/generate', {
        tool: selectedTool, base_url: baseUrl, api_key: apiKey, model: model || undefined,
      })
      if (res.data?.success) {
        setResult(res.data.data.config || res.data.data)
        toast.success(t('Config generated'))
      } else {
        toast.error(res.data?.message || t('Generation failed'))
      }
    } catch {
      toast.error(t('Generation failed'))
    } finally {
      setGenerating(false)
    }
  }

  const copyToClipboard = (text: string) => {
    navigator.clipboard.writeText(text)
    toast.success(t('Copied to clipboard'))
  }

  return (
    <Card>
      <CardHeader>
        <CardTitle>{t('CLI Tools Config Generator')}</CardTitle>
        <CardDescription>{t('Generate connection config for Claude Code / Cursor / Copilot and other AI coding tools')}</CardDescription>
      </CardHeader>
      <CardContent className="space-y-4">
        <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
          <div className="space-y-2">
            <Label>{t('Target Tool')}</Label>
            {loading ? (
              <div className="text-muted-foreground text-sm py-2">{t('Loading tools...')}</div>
            ) : (
              <Select value={selectedTool} onValueChange={setSelectedTool}>
                <SelectTrigger className="w-full">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  {tools.map((tool) => (
                    <SelectItem key={tool.id} value={tool.id}>
                      {tool.icon} {tool.name} ({tool.envVar})
                    </SelectItem>
                  ))}
                  {tools.length === 0 && (
                    <>
                      <SelectItem value="claude">Claude Code</SelectItem>
                      <SelectItem value="cursor">Cursor</SelectItem>
                      <SelectItem value="copilot">GitHub Copilot</SelectItem>
                    </>
                  )}
                </SelectContent>
              </Select>
            )}
          </div>
          <div className="space-y-2">
            <Label>{t('API Gateway URL')}</Label>
            <Input value={baseUrl} onChange={(e) => setBaseUrl(e.target.value)} placeholder="https://your-api.com" />
          </div>
          <div className="space-y-2">
            <Label>API Key</Label>
            <Input value={apiKey} onChange={(e) => setApiKey(e.target.value)} placeholder="sk-..." type="password" />
          </div>
          <div className="space-y-2">
            <Label>{t('Default Model')} ({t('optional')})</Label>
            <Input value={model} onChange={(e) => setModel(e.target.value)} placeholder="claude-sonnet-4-20250514" />
          </div>
        </div>

        <Button onClick={generate} className="w-full" disabled={generating}>
          {generating ? t('Generating...') : t('Generate Config')}
        </Button>

        {result && (
          <div className="space-y-2">
            <div className="flex justify-between items-center">
              <Label>{t('Generated Config')}</Label>
              <Button variant="outline" size="sm" onClick={() => copyToClipboard(JSON.stringify(result, null, 2))}>
                📋 {t('Copy JSON')}
              </Button>
            </div>

            {/* Show env vars if present */}
            {result.env && (
              <div className="space-y-1">
                <Label className="text-xs text-muted-foreground">{t('Environment Variables')}</Label>
                {Object.entries(result.env).map(([key, val]) => (
                  <div key={key} className="flex items-center gap-2 p-2 bg-muted rounded text-xs font-mono">
                    <span className="font-bold text-blue-600">{key}</span>
                    <span>=</span>
                    <span className="flex-1 truncate">{String(val)}</span>
                    <Button variant="ghost" size="sm" onClick={() => copyToClipboard(`export ${key}="${val}"`)}>
                      📋
                    </Button>
                  </div>
                ))}
              </div>
            )}

            {/* Show VS Code settings if present */}
            {result.vscode_settings && (
              <div className="space-y-1">
                <Label className="text-xs text-muted-foreground">{t('VS Code Settings')}</Label>
                {Object.entries(result.vscode_settings).map(([key, val]) => (
                  <div key={key} className="flex items-center gap-2 p-2 bg-muted rounded text-xs font-mono">
                    <span className="font-bold text-purple-600">{key}</span>
                    <span>:</span>
                    <span className="flex-1 truncate">{JSON.stringify(val)}</span>
                  </div>
                ))}
              </div>
            )}

            {/* Show settings file content if present */}
            {result.settings_contents && (
              <div className="space-y-1">
                <Label className="text-xs text-muted-foreground">
                  {t('Config File Content')} {result.settings_file && <span className="text-blue-500">({result.settings_file})</span>}
                </Label>
                <Textarea
                  value={JSON.stringify(result.settings_contents, null, 2)}
                  readOnly
                  rows={6}
                  className="font-mono text-xs"
                />
              </div>
            )}

            {/* Raw JSON fallback */}
            {!result.env && !result.vscode_settings && !result.settings_contents && (
              <Textarea
                value={JSON.stringify(result, null, 2)}
                readOnly
                rows={8}
                className="font-mono text-xs"
              />
            )}
          </div>
        )}
      </CardContent>
    </Card>
  )
}
