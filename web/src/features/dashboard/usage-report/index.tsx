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

import { Button } from '@/components/ui/button'
import { Skeleton } from '@/components/ui/skeleton'
import dayjs from '@/lib/dayjs'

import { DailyTrendChart } from './components/daily-trend-chart'
import { DepartmentTable } from './components/department-table'
import { OverviewCards } from './components/overview-cards'
import { PeriodSwitcher } from './components/period-switcher'
import { UserTable } from './components/user-table'
import { useUsageReport } from './hooks/use-usage-report'
import { exportUsageReportPptx } from './lib/export-ppt'
import type { UsageReportPeriod } from './types'

export function UsageReportSection() {
  const { t } = useTranslation()
  const [period, setPeriod] = useState<UsageReportPeriod>('week')
  const [anchor, setAnchor] = useState<string>(() =>
    dayjs().format('YYYY-MM-DD')
  )
  const [exporting, setExporting] = useState(false)
  const { data, isLoading, isError } = useUsageReport(period, anchor)

  const handleExport = async () => {
    if (!data) return
    setExporting(true)
    try {
      await exportUsageReportPptx(data, t)
    } finally {
      setExporting(false)
    }
  }

  let content: React.ReactNode
  if (isLoading) {
    content = <Skeleton className='h-40 w-full' />
  } else if (isError || !data) {
    content = (
      <div className='text-muted-foreground py-8 text-center' role='alert'>
        {t('Failed to load usage report')}
      </div>
    )
  } else {
    content = (
      <>
        <div className='text-muted-foreground text-sm'>
          {data.current.start_date} ~ {data.current.end_date}
          {' · '}
          {t('vs previous period')} {data.previous.label}
        </div>
        <OverviewCards overview={data.overview} />
        <DailyTrendChart daily={data.daily} />
        <DepartmentTable departments={data.departments} />
        <UserTable users={data.users} />
      </>
    )
  }

  return (
    <div className='space-y-6'>
      <div className='flex items-center justify-between'>
        <PeriodSwitcher
          period={period}
          anchor={anchor}
          onChange={(next) => {
            if (next.period) setPeriod(next.period)
            if (next.anchor) setAnchor(next.anchor)
          }}
        />
        <Button onClick={handleExport} disabled={!data || exporting}>
          {exporting ? t('Exporting...') : t('Export PPT')}
        </Button>
      </div>
      {content}
    </div>
  )
}
