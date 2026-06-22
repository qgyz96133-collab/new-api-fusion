import { useState, useEffect } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Badge } from '@/components/ui/badge'
import { Input } from '@/components/ui/input'
import { Textarea } from '@/components/ui/textarea'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { api } from '@/lib/api'

interface KiroConnectProps {
  onChannelCreated?: (channelId: number) => void
}

export function KiroConnect({ onChannelCreated }: KiroConnectProps) {
  const { t } = useTranslation()
  const [mode, setMode] = useState<'device' | 'paste'>('device')
  
  // Device code flow state
  const [step, setStep] = useState<'idle' | 'waiting' | 'creating' | 'done'>('idle')
  const [sessionId, setSessionId] = useState('')
  const [userCode, setUserCode] = useState('')
  const [verificationUri, setVerificationUri] = useState('')
  const [error, setError] = useState('')
  const [accountInfo, setAccountInfo] = useState<any>({ action: 'created', account_count: 1 })

  // Paste token state
  const [pasteToken, setPasteToken] = useState('')

  const startDeviceAuth = async () => {
    setStep('waiting')
    setError('')
    try {
      const res = await api.post('/api/user/kiro/auth/start')
      if (res.data?.success) {
        const data = res.data.data
        setSessionId(data.session_id)
        setUserCode(data.user_code)
        setVerificationUri(data.verification_uri_complete || data.verification_uri)
        if (data.verification_uri_complete) {
          window.open(data.verification_uri_complete, '_blank')
        }
      } else {
        setError(res.data?.message || 'Failed to start auth')
        setStep('idle')
      }
    } catch (e: any) {
      setError(e.response?.data?.message || e.message)
      setStep('idle')
    }
  }

  useEffect(() => {
    if (step !== 'waiting' || !sessionId) return
    const interval = setInterval(async () => {
      try {
        const res = await api.get(`/api/user/kiro/auth/poll/${sessionId}`)
        if (res.data?.success && res.data.data.status === 'authorized') {
          clearInterval(interval)
          setStep('creating')
          await createChannel(sessionId)
        } else if (res.data?.data?.status === 'timeout') {
          clearInterval(interval)
          setError('授权超时，请重试')
          setStep('idle')
        }
      } catch {}
    }, 5000)
    const timeout = setTimeout(() => {
      clearInterval(interval)
      if (step === 'waiting') { setError('超时'); setStep('idle') }
    }, 10 * 60 * 1000)
    return () => { clearInterval(interval); clearTimeout(timeout) }
  }, [step, sessionId])

  const createChannel = async (sid: string) => {
    try {
      const res = await api.post('/api/user/kiro/auth/create-channel', { session_id: sid })
      if (res.data?.success) {
        setAccountInfo(res.data.data)
        setStep('done')
        toast.success(`Kiro ${res.data.data.action}: ${res.data.data.account_count} 个账号`)
        onChannelCreated?.(res.data.data.channel_id)
      } else {
        setError(res.data?.message || 'Failed')
        setStep('idle')
      }
    } catch (e: any) { setError(e.message); setStep('idle') }
  }

  const handlePasteImport = async () => {
    if (!pasteToken.trim()) { toast.error('请输入 Token'); return }
    try {
      const importRes = await api.post('/api/user/kiro/auth/import', { token: pasteToken.trim() })
      if (importRes.data?.success) {
        await createChannel(importRes.data.data.session_id)
        setPasteToken('')
      }
    } catch (e: any) { toast.error(e.message) }
  }

  return (
    <Card className="border-dashed">
      <CardHeader className="pb-3">
        <CardTitle className="flex items-center gap-2 text-base">
          <span>🔷</span>
          <span>Kiro AI (Free)</span>
          {step === 'done' && <Badge className="bg-green-500">Connected</Badge>}
          {step === 'waiting' && <Badge className="bg-yellow-500 animate-pulse">Waiting...</Badge>}
        </CardTitle>
      </CardHeader>
      <CardContent className="space-y-3">
        <Tabs value={mode} onValueChange={(v) => setMode(v as any)} className="w-full">
          <TabsList className="grid w-full grid-cols-2">
            <TabsTrigger value="device">AWS 设备码授权</TabsTrigger>
            <TabsTrigger value="paste">粘贴 Token</TabsTrigger>
          </TabsList>

          <TabsContent value="device" className="space-y-3">
            {step === 'idle' && (
              <>
                <p className="text-xs text-muted-foreground">通过 AWS Builder ID 免费授权，获取 Claude 系列模型访问权限</p>
                {error && <p className="text-xs text-red-500">{error}</p>}
                <Button onClick={startDeviceAuth} className="w-full">🔗 Connect Kiro (AWS)</Button>
              </>
            )}
            {step === 'waiting' && (
              <div className="space-y-2">
                <div className="p-3 bg-muted rounded text-center">
                  <p className="text-xs text-muted-foreground mb-1">请在打开的页面中输入验证码：</p>
                  <p className="text-2xl font-mono font-bold tracking-widest">{userCode}</p>
                </div>
                {verificationUri && (
                  <a href={verificationUri} target="_blank" rel="noopener" className="text-xs text-blue-500 hover:underline block text-center">
                    打开授权页面 ↗
                  </a>
                )}
                <p className="text-xs text-muted-foreground text-center animate-pulse">等待授权中...</p>
              </div>
            )}
            {step === 'creating' && <p className="text-sm animate-pulse">⚙️ 创建渠道中...</p>}
            {step === 'done' && (
              <div className="space-y-2">
                <p className="text-sm text-green-600">
                  ✅ {accountInfo.action === 'created' ? 'Kiro 渠道已创建' : '账号已添加'}
                </p>
                <Badge variant="secondary">共 {accountInfo.account_count} 个账号</Badge>
                <Button variant="outline" onClick={() => { setStep('idle'); setError('') }} className="w-full">
                  🔗 继续添加账号
                </Button>
              </div>
            )}
          </TabsContent>

          <TabsContent value="paste" className="space-y-3">
            <p className="text-xs text-muted-foreground">
              粘贴 Kiro OAuth Token（从 .kiro/oauth_creds.json 获取 access_token）
            </p>
            <Textarea
              value={pasteToken}
              onChange={(e) => setPasteToken(e.target.value)}
              rows={3}
              className="font-mono text-xs"
              placeholder="eyJraWQiOiJ... (access_token)"
            />
            <Button onClick={handlePasteImport} disabled={!pasteToken.trim()} className="w-full">
              📋 导入并创建渠道
            </Button>
          </TabsContent>
        </Tabs>
      </CardContent>
    </Card>
  )
}
