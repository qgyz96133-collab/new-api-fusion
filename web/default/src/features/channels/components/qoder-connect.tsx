import { useState, useEffect, useCallback } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Badge } from '@/components/ui/badge'
import { api } from '@/lib/api'

interface QoderConnectProps {
  onChannelCreated?: (channelId: number) => void
}

export function QoderConnect({ onChannelCreated }: QoderConnectProps) {
  const { t } = useTranslation()
  const [step, setStep] = useState<'idle' | 'authorizing' | 'polling' | 'creating' | 'done'>('idle')
  const [sessionId, setSessionId] = useState('')
  const [loginUrl, setLoginUrl] = useState('')
  const [nonce, setNonce] = useState('')
  const [error, setError] = useState('')
  const [accountInfo, setAccountInfo] = useState<any>({ action: 'created', account_count: 1 })

  const startAuth = async () => {
    setStep('authorizing')
    setError('')
    try {
      const res = await api.post('/api/user/qoder/auth/start')
      if (res.data?.success) {
        setSessionId(res.data.data.session_id)
        setLoginUrl(res.data.data.login_url)
        setNonce(res.data.data.nonce)
        setStep('polling')
        // Open login URL in new tab
        window.open(res.data.data.login_url, '_blank')
      } else {
        setError(res.data?.message || 'Failed to start auth')
        setStep('idle')
      }
    } catch (e: any) {
      setError(e.message)
      setStep('idle')
    }
  }

  // Poll for token
  useEffect(() => {
    if (step !== 'polling' || !sessionId) return

    const interval = setInterval(async () => {
      try {
        const res = await api.get(`/api/user/qoder/auth/poll/${sessionId}`)
        if (res.data?.success && res.data.data.status === 'authorized') {
          clearInterval(interval)
          setStep('creating')
          // Auto-create channel
          await createChannel()
        }
      } catch {}
    }, 3000) // Poll every 3 seconds

    // Timeout after 5 minutes
    const timeout = setTimeout(() => {
      clearInterval(interval)
      if (step === 'polling') {
        setError('Authorization timeout - please try again')
        setStep('idle')
      }
    }, 5 * 60 * 1000)

    return () => {
      clearInterval(interval)
      clearTimeout(timeout)
    }
  }, [step, sessionId])

  const createChannel = async () => {
    try {
      const res = await api.post('/api/user/qoder/auth/create-channel', {
        session_id: sessionId,
      })
      if (res.data?.success) {
        setStep('done')
        setAccountInfo(res.data.data)
        toast.success(`Qoder ${res.data.data.action}: ${res.data.data.account_count} 个账号`)
        onChannelCreated?.(res.data.data.channel_id)
      } else {
        setError(res.data?.message || 'Failed to create channel')
        setStep('idle')
      }
    } catch (e: any) {
      setError(e.message)
      setStep('idle')
    }
  }

  return (
    <Card className="border-dashed">
      <CardHeader className="pb-3">
        <CardTitle className="flex items-center gap-2 text-base">
          <span>💧</span>
          <span>Qoder Auto-Connect</span>
          {step === 'done' && <Badge className="bg-green-500">Connected</Badge>}
          {step === 'polling' && <Badge className="bg-yellow-500 animate-pulse">Waiting...</Badge>}
        </CardTitle>
      </CardHeader>
      <CardContent className="space-y-3">
        {step === 'idle' && (
          <>
            <p className="text-sm text-muted-foreground">
              Click below to authorize Qoder and automatically create a channel.
            </p>
            {error && <p className="text-sm text-red-500">{error}</p>}
            <Button onClick={startAuth} className="w-full">
              🔗 Connect Qoder
            </Button>
          </>
        )}

        {step === 'polling' && (
          <div className="space-y-2">
            <div className="text-sm">
              <p className="font-medium">Waiting for authorization...</p>
              <p className="text-muted-foreground">
                A new tab should have opened. Please log in to Qoder there.
              </p>
              {loginUrl && (
                <a href={loginUrl} target="_blank" rel="noopener" className="text-blue-500 text-xs break-all hover:underline">
                  Open login page ↗
                </a>
              )}
            </div>
            <div className="flex items-center gap-2 text-xs text-muted-foreground">
              <span className="animate-spin">⏳</span>
              <span>Polling for token... (nonce: {nonce})</span>
            </div>
          </div>
        )}

        {step === 'creating' && (
          <div className="flex items-center gap-2 text-sm">
            <span className="animate-spin">⚙️</span>
            <span>Creating channel...</span>
          </div>
        )}

        {step === 'done' && (
          <div className="space-y-2">
            <p className="text-sm text-green-600">✅ {accountInfo.action === 'created' ? 'Qoder 渠道已创建' : accountInfo.action === 'appended' ? '账号已添加' : '账号已更新'}</p>
            <div className="flex items-center gap-2 text-xs">
              <Badge variant="secondary">共 {accountInfo.account_count} 个账号</Badge>
              <Badge variant="outline">轮询模式</Badge>
            </div>
            <p className="text-xs text-muted-foreground">
              {accountInfo.action === 'appended' 
                ? '新账号已堆叠到现有渠道，自动轮询切换' 
                : accountInfo.action === 'updated'
                ? '已更新该账号的 Token'
                : '首个账号，继续添加以堆叠更多账号'}
            </p>
            <Button variant="outline" onClick={() => setStep('idle')} className="w-full">
              🔗 继续添加账号
            </Button>
          </div>
        )}
      </CardContent>
    </Card>
  )
}
