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
import { VChart } from '@visactor/react-vchart'
import { useMemo } from 'react'
import { useTranslation } from 'react-i18next'

import { useTheme } from '@/context/theme-provider'
import { quotaUnitsToDollars } from '@/lib/format'
import { VCHART_OPTION } from '@/lib/vchart'

import type { UsageReportDaily } from '../types'

interface DailyTrendChartProps {
  daily: UsageReportDaily[]
}

export function DailyTrendChart(props: DailyTrendChartProps) {
  const { t } = useTranslation()
  const { resolvedTheme } = useTheme()

  const spec = useMemo(
    () => ({
      type: 'common',
      data: [
        {
          id: 'usage',
          values: props.daily.map((item) => ({
            date: item.date.slice(5),
            requests: item.requests,
            cost: quotaUnitsToDollars(item.quota),
          })),
        },
      ],
      series: [
        {
          type: 'bar',
          id: 'requests',
          xField: 'date',
          yField: 'requests',
          data: { id: 'usage' },
        },
        {
          type: 'line',
          id: 'cost',
          xField: 'date',
          yField: 'cost',
          data: { id: 'usage' },
        },
      ],
      axes: [
        { orient: 'left', title: { visible: true, text: t('Requests') } },
        { orient: 'right', title: { visible: true, text: t('Cost') } },
        { orient: 'bottom', label: { autoRotate: true } },
      ],
    }),
    [props.daily, t]
  )

  return (
    <div className='h-80' role='img' aria-label={t('Daily trend')}>
      <VChart
        spec={{
          ...spec,
          theme: resolvedTheme === 'dark' ? 'dark' : 'light',
          background: 'transparent',
        }}
        option={VCHART_OPTION}
      />
    </div>
  )
}
