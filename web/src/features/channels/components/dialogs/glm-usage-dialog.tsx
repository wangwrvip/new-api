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
import { RefreshCw } from 'lucide-react'
import { useTranslation } from 'react-i18next'

import { Dialog } from '@/components/dialog'
import { StatusBadge } from '@/components/status-badge'
import { Button } from '@/components/ui/button'
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'
import { Progress } from '@/components/ui/progress'
import dayjs from '@/lib/dayjs'
import { formatTimestampToDate } from '@/lib/format'
import { cn } from '@/lib/utils'

import {
  getGlmRemainingVariant,
  translateGlmUsageMessage,
} from '../../lib'
import type { GlmUsageResult, GlmUsageSlot } from '../../types'

type GlmUsageDialogProps = {
  open: boolean
  onOpenChange: (open: boolean) => void
  channelName?: string
  channelId?: number
  channelDisplayName?: string
  channelDisplayId?: string
  response: GlmUsageResult | null
  onRefresh?: () => void | Promise<void>
  isRefreshing?: boolean
}

const remainingTextClassName: Record<string, string> = {
  danger: 'text-destructive',
  warning: 'text-warning',
  info: 'text-info',
}

function formatResetsIn(
  nextResetMs: number | null | undefined,
  t: (key: string) => string
): string {
  if (!nextResetMs) {
    return '-'
  }
  const secondsLeft = dayjs(nextResetMs).diff(dayjs(), 'second')
  if (secondsLeft <= 0) {
    return t('Expired')
  }
  const days = Math.floor(secondsLeft / (24 * 60 * 60))
  const hours = Math.floor((secondsLeft % (24 * 60 * 60)) / 3600)
  const minutes = Math.floor((secondsLeft % 3600) / 60)
  if (days > 0) {
    return `${days} ${t('days')} ${hours}${t('h')}`
  }
  if (hours > 0) {
    return `${hours}${t('h')} ${minutes}${t('m')}`
  }
  return `${minutes}${t('m')}`
}

function GlmUsageWindow(props: { title: string; slot?: GlmUsageSlot | null }) {
  const { t } = useTranslation()
  const slot = props.slot
  const remaining = slot ? slot.remaining_pct : null
  const variant = remaining != null ? getGlmRemainingVariant(remaining) : 'info'
  const resetAt = slot
    ? formatTimestampToDate(slot.next_reset_ms, 'milliseconds')
    : '-'

  return (
    <Card size='sm' className='gap-0 py-0'>
      <CardHeader className='p-3 pb-2'>
        <div className='flex items-start justify-between gap-3'>
          <div className='min-w-0'>
            <CardTitle className='text-sm font-semibold'>
              {props.title}
            </CardTitle>
            <CardDescription className='mt-1 text-xs'>
              {t('Remaining')}
            </CardDescription>
          </div>
          <div className='shrink-0 text-right'>
            <div
              className={cn(
                'text-xl leading-none font-semibold tabular-nums',
                remainingTextClassName[variant]
              )}
            >
              {remaining != null ? `${remaining}%` : '-'}
            </div>
            <div className='text-muted-foreground mt-1 text-[11px]'>
              {t('Used')} {slot ? `${slot.used_pct}%` : '-'}
            </div>
          </div>
        </div>
      </CardHeader>
      <CardContent className='p-3 pt-0'>
        <Progress
          value={remaining ?? 0}
          aria-label={`${props.title}: ${remaining != null ? `${remaining}%` : '-'}`}
          className='mt-1'
        />
        <div className='mt-3 grid grid-cols-1 gap-2 text-xs sm:grid-cols-2'>
          <div className='min-w-0'>
            <div className='text-muted-foreground text-[11px]'>
              {t('Reset at:')}
            </div>
            <div className='break-all tabular-nums'>{resetAt}</div>
          </div>
          <div className='min-w-0 sm:text-right'>
            <div className='text-muted-foreground text-[11px]'>
              {t('Resets in:')}
            </div>
            <div className='tabular-nums'>
              {formatResetsIn(slot?.next_reset_ms, t)}
            </div>
          </div>
        </div>
      </CardContent>
    </Card>
  )
}

export function GlmUsageDialog(props: GlmUsageDialogProps) {
  const { t } = useTranslation()
  const channelLabelName = props.channelDisplayName ?? props.channelName ?? '-'
  let channelLabelId = ''
  if (props.channelDisplayId != null) {
    channelLabelId = ` (#${props.channelDisplayId})`
  } else if (props.channelId) {
    channelLabelId = ` (#${props.channelId})`
  }
  const channelLabel = `${channelLabelName}${channelLabelId}`
  // The server returns fixed messages for upstream failures; it never sends
  // the original error. The dynamic upstream status line needs its template
  // key so translated interfaces keep the numeric status code.
  const errorMessage =
    props.response?.success === false
      ? translateGlmUsageMessage(
          props.response.message?.trim() || 'Failed to fetch usage',
          t
        )
      : ''

  return (
    <Dialog
      open={props.open}
      onOpenChange={props.onOpenChange}
      title={t('GLM Usage')}
      contentClassName='sm:max-w-[640px]'
      contentHeight='auto'
      bodyClassName='flex flex-col gap-4'
      footer={
        <Button
          type='button'
          variant='outline'
          onClick={() => props.onOpenChange(false)}
        >
          {t('Close')}
        </Button>
      }
    >
      <div className='flex flex-col gap-4'>
        {errorMessage && (
          <div className='rounded-lg border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-700 dark:border-red-800 dark:bg-red-950/30 dark:text-red-400'>
            {errorMessage}
          </div>
        )}

        <div className='flex flex-wrap items-center justify-between gap-2'>
          <div className='flex min-w-0 flex-wrap items-center gap-2'>
            {props.response?.level ? (
              <StatusBadge
                label={`${t('Plan')} ${props.response.level}`}
                variant='info'
                copyable={false}
              />
            ) : null}
            <StatusBadge
              label={channelLabel}
              variant='neutral'
              copyable={false}
            />
          </div>
          {props.onRefresh ? (
            <Button
              type='button'
              variant='outline'
              size='sm'
              onClick={() => void props.onRefresh?.()}
              disabled={Boolean(props.isRefreshing)}
            >
              <RefreshCw data-icon='inline-start' />
              {t('Refresh')}
            </Button>
          ) : null}
        </div>

        <div className='grid grid-cols-1 gap-3 md:grid-cols-2'>
          <GlmUsageWindow
            title={t('5-Hour Window')}
            slot={props.response?.five_h}
          />
          <GlmUsageWindow
            title={t('Weekly Window')}
            slot={props.response?.weekly}
          />
        </div>

        {props.response == null && (
          <p className='text-muted-foreground text-sm'>
            {props.isRefreshing ? t('Loading...') : t('No usage data')}
          </p>
        )}
      </div>
    </Dialog>
  )
}
