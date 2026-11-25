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
import isoWeek from 'dayjs/plugin/isoWeek'
import { ChevronLeft, ChevronRight } from 'lucide-react'
import { useTranslation } from 'react-i18next'

import { Button } from '@/components/ui/button'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Tabs, TabsList, TabsTrigger } from '@/components/ui/tabs'
import dayjs from '@/lib/dayjs'

import type { UsageReportPeriod } from '../types'

// Backend weekly windows are ISO weeks (Monday start); register the plugin on
// the shared dayjs instance so option labels and anchors stay in sync.
dayjs.extend(isoWeek)

interface PeriodSwitcherProps {
  period: UsageReportPeriod
  anchor: string
  onChange: (next: { period?: UsageReportPeriod; anchor?: string }) => void
}

interface PeriodOption {
  value: string // 周期锚点 YYYY-MM-DD
  label: string // '2026-08' / '2026-W36'
}

// 生成含当前周期在内的最近 count 个周期选项（周报 26 周、月报 12 个月）。
// 锚点取周期内稳定日期（周取周一、月取 15 日），与后端周期判定一致：
// 选中后本期即该周期、上期为上一周期，可快速跳到任意历史周期导出。
function buildPeriodOptions(
  period: UsageReportPeriod,
  anchor: string,
  count: number
): PeriodOption[] {
  const base = dayjs(anchor)
  const options: PeriodOption[] = []
  for (let i = 0; i < count; i++) {
    const cursor =
      period === 'week'
        ? base.subtract(7 * i, 'day')
        : base.subtract(i, 'month')
    const anchorDate =
      period === 'week' ? cursor.startOf('isoWeek') : cursor.date(15)
    const label =
      period === 'week'
        ? `${anchorDate.isoWeekYear()}-W${String(anchorDate.isoWeek()).padStart(2, '0')}`
        : cursor.format('YYYY-MM')
    options.push({ value: anchorDate.format('YYYY-MM-DD'), label })
  }
  return options
}

export function PeriodSwitcher(props: PeriodSwitcherProps) {
  const { t } = useTranslation()
  const options = buildPeriodOptions(
    props.period,
    props.anchor,
    props.period === 'week' ? 26 : 12
  )
  const currentAnchor = options[0]?.value ?? props.anchor

  const shiftAnchor = (direction: 1 | -1) => {
    const base = dayjs(props.anchor)
    const next =
      props.period === 'week'
        ? base.add(7 * direction, 'day')
        : base.add(direction, 'month')
    props.onChange({ anchor: next.format('YYYY-MM-DD') })
  }

  return (
    <div className='flex items-center gap-2'>
      <Button
        variant='outline'
        size='icon'
        aria-label={t('Previous period')}
        onClick={() => shiftAnchor(-1)}
      >
        <ChevronLeft aria-hidden='true' />
      </Button>
      <Tabs
        value={props.period}
        onValueChange={(value) =>
          props.onChange({ period: value as UsageReportPeriod })
        }
      >
        <TabsList>
          <TabsTrigger value='week'>{t('Weekly')}</TabsTrigger>
          <TabsTrigger value='month'>{t('Monthly')}</TabsTrigger>
        </TabsList>
      </Tabs>
      <Select
        value={currentAnchor}
        onValueChange={(value) => {
          if (value) props.onChange({ anchor: value })
        }}
      >
        <SelectTrigger
          size='sm'
          aria-label={t('Select period')}
          className='w-32'
        >
          <SelectValue />
        </SelectTrigger>
        <SelectContent>
          {options.map((option) => (
            <SelectItem key={option.value} value={option.value}>
              {option.label}
            </SelectItem>
          ))}
        </SelectContent>
      </Select>
      <Button
        variant='outline'
        size='icon'
        aria-label={t('Next period')}
        onClick={() => shiftAnchor(1)}
      >
        <ChevronRight aria-hidden='true' />
      </Button>
    </div>
  )
}
