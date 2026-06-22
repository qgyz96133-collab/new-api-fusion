/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { useQueryClient } from '@tanstack/react-query'
import { toast } from 'sonner'
import { SectionPageLayout } from '@/components/layout'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import {
  Collapsible,
  CollapsibleTrigger,
  CollapsibleContent,
} from '@/components/ui/collapsible'
import { ChannelsDialogs } from './components/channels-dialogs'
import { ChannelsPrimaryButtons } from './components/channels-primary-buttons'
import { ChannelsProvider } from './components/channels-provider'
import { ChannelsTable } from './components/channels-table'
import { QoderConnect } from './components/qoder-connect'
import { KiroConnect } from './components/kiro-connect'
import { api } from '@/lib/api'

function FreeTierProviders({ onRefresh }: { onRefresh: () => void }) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const [addingMimo, setAddingMimo] = useState(false)

  const addMimoChannel = async () => {
    setAddingMimo(true)
    try {
      const res = await api.post('/api/channel/', {
        mode: 'single',
        channel: {
          type: 66,
          name: 'MiMo Code Free (auto)',
          key: 'free',
          base_url: 'https://api.xiaomimimo.com/api/free-ai/openai/chat',
          models: 'mimo-auto',
          model_mapping: '',
          status: 1,
        },
      })
      if (res.data?.success || res.data?.data) {
        toast.success('MiMo Code Free channel created!')
        queryClient.invalidateQueries({ queryKey: ['channels'] })
        onRefresh()
      } else {
        toast.error(res.data?.message || 'Failed to create channel')
      }
    } catch (e: any) {
      toast.error(e.message || 'Failed to create MiMo channel')
    } finally {
      setAddingMimo(false)
    }
  }

  return (
    <div className="grid grid-cols-1 md:grid-cols-3 gap-3">
      {/* Qoder Auto-Connect */}
      <QoderConnect onChannelCreated={() => {
        queryClient.invalidateQueries({ queryKey: ['channels'] })
        onRefresh()
      }} />

      {/* MiMo Free One-Click */}
      <Card className="border-dashed">
        <CardHeader className="pb-3">
          <CardTitle className="flex items-center gap-2 text-base">
            <span>🤖</span>
            <span>MiMo Code Free</span>
          </CardTitle>
        </CardHeader>
        <CardContent className="space-y-3">
          <p className="text-sm text-muted-foreground">
            Free tier - no API key needed. One-click to add.
          </p>
          <Button onClick={addMimoChannel} disabled={addingMimo} className="w-full">
            {addingMimo ? '⏳ Creating...' : '➕ Add MiMo Free Channel'}
          </Button>
        </CardContent>
      </Card>

      {/* Kiro Auto-Connect */}
      <KiroConnect onChannelCreated={() => {
        queryClient.invalidateQueries({ queryKey: ['channels'] })
        onRefresh()
      }} />
    </div>
  )
}

function CollapsibleFreeTierProviders({ onRefresh }: { onRefresh: () => void }) {
  const { t } = useTranslation()
  const [open, setOpen] = useState(false)

  return (
    <Collapsible open={open} onOpenChange={setOpen} className="shrink-0 mb-2">
      <CollapsibleTrigger className="flex w-full items-center justify-between rounded-md border border-dashed bg-muted/30 px-3 py-1.5 text-sm font-medium hover:bg-muted/50 transition-colors">
        <span className="flex items-center gap-2">
          <span>⚡</span>
          <span>{t('Quick Add Channels')}</span>
          <span className="text-xs text-muted-foreground">
            (Qoder / MiMo / Kiro)
          </span>
        </span>
        <span className={`transition-transform duration-200 ${open ? 'rotate-180' : ''}`}>
          ▼
        </span>
      </CollapsibleTrigger>
      <CollapsibleContent className="mt-2">
        <FreeTierProviders onRefresh={onRefresh} />
      </CollapsibleContent>
    </Collapsible>
  )
}

export function Channels() {
  const { t } = useTranslation()
  const [refreshKey, setRefreshKey] = useState(0)
  return (
    <ChannelsProvider>
      <SectionPageLayout fixedContent>
        <SectionPageLayout.Title>{t('Channels')}</SectionPageLayout.Title>
        <SectionPageLayout.Actions>
          <ChannelsPrimaryButtons />
        </SectionPageLayout.Actions>
        <SectionPageLayout.Content>
          {/* Flex column wrapper: ensures collapsible + table share the
              available height correctly. The collapsible is shrink-0
              (fixed to its content height), and the table fills the
              remaining space via flex-1 min-h-0. */}
          <div className="flex h-full min-h-0 flex-col">
            <CollapsibleFreeTierProviders onRefresh={() => setRefreshKey(k => k + 1)} />
            <div className="min-h-0 flex-1">
              <ChannelsTable key={refreshKey} />
            </div>
          </div>
        </SectionPageLayout.Content>
      </SectionPageLayout>

      <ChannelsDialogs />
    </ChannelsProvider>
  )
}
