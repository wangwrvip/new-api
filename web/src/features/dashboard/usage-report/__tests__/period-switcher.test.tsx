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
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { render, renderHook, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import type { ReactNode } from 'react'
import { describe, expect, it, vi } from 'vitest'

import { api } from '@/lib/api'

import { PeriodSwitcher } from '../components/period-switcher'
import { useUsageReport } from '../hooks/use-usage-report'
import type { UsageReportPeriod } from '../types'

function show(
  period: UsageReportPeriod,
  anchor: string,
  onChange = vi.fn<
    (next: { period?: UsageReportPeriod; anchor?: string }) => void
  >()
) {
  render(<PeriodSwitcher period={period} anchor={anchor} onChange={onChange} />)
  return onChange
}

function queryWrapper(queryClient: QueryClient) {
  return function QueryWrapper(props: { children: ReactNode }) {
    return (
      <QueryClientProvider client={queryClient}>
        {props.children}
      </QueryClientProvider>
    )
  }
}

describe('PeriodSwitcher', () => {
  it('anchors a Sunday to the preceding ISO Monday when choosing a week', async () => {
    const user = userEvent.setup()
    const onChange = show('week', '2026-01-11')
    const trigger = screen.getByRole('combobox', { name: 'Select period' })

    // 2026-01-11 is a Sunday; its ISO week starts on Monday 2026-01-05.
    expect(trigger).toHaveTextContent('2026-01-05')
    await user.click(trigger)
    expect(screen.getByRole('option', { name: '2026-W02' })).toBeVisible()
    await user.click(screen.getByRole('option', { name: '2026-W01' }))

    expect(onChange).toHaveBeenCalledWith({ anchor: '2025-12-29' })
  })

  it('keeps ISO week-year and week labels across the new-year boundary', async () => {
    const user = userEvent.setup()
    const onChange = show('week', '2026-01-04')
    const trigger = screen.getByRole('combobox', { name: 'Select period' })

    expect(trigger).toHaveTextContent('2025-12-29')
    await user.click(trigger)
    expect(screen.getByRole('option', { name: '2025-W52' })).toBeVisible()
    expect(screen.getByRole('option', { name: '2025-W51' })).toBeVisible()

    await user.click(screen.getByRole('option', { name: '2025-W52' }))
    expect(onChange).toHaveBeenCalledWith({ anchor: '2025-12-22' })
  })

  it('shifts the anchor by exactly seven days in weekly mode', async () => {
    const user = userEvent.setup()
    const onChange = show('week', '2026-01-15')

    await user.click(screen.getByRole('button', { name: 'Previous period' }))
    expect(onChange).toHaveBeenLastCalledWith({ anchor: '2026-01-08' })
    await user.click(screen.getByRole('button', { name: 'Next period' }))
    expect(onChange).toHaveBeenLastCalledWith({ anchor: '2026-01-22' })
    expect(onChange).toHaveBeenCalledTimes(2)
  })

  it('shifts the anchor by exactly one calendar month in monthly mode', async () => {
    const user = userEvent.setup()
    const onChange = show('month', '2026-01-15')

    await user.click(screen.getByRole('button', { name: 'Previous period' }))
    expect(onChange).toHaveBeenLastCalledWith({ anchor: '2025-12-15' })
    await user.click(screen.getByRole('button', { name: 'Next period' }))
    expect(onChange).toHaveBeenLastCalledWith({ anchor: '2026-02-15' })
    expect(onChange).toHaveBeenCalledTimes(2)
  })
})

// The anchor and the timezone offset are two halves of the same backend
// contract: the backend interprets YYYY-MM-DD in a fixed zone given by
// tz_offset (east-positive minutes), the opposite of JS getTimezoneOffset.
describe('useUsageReport timezone offset', () => {
  it.each([
    { browserOffset: -480, sentOffset: 480 },
    { browserOffset: 420, sentOffset: -420 },
  ])(
    'sends east-positive $sentOffset for browser offset $browserOffset',
    async ({ browserOffset, sentOffset }) => {
      vi.spyOn(Date.prototype, 'getTimezoneOffset').mockReturnValue(
        browserOffset
      )
      const get = vi.spyOn(api, 'get').mockResolvedValue({
        data: { success: true, data: {} },
      })
      const queryClient = new QueryClient({
        defaultOptions: { queries: { retry: false } },
      })
      const { result } = renderHook(
        () => useUsageReport('week', '2026-01-04'),
        { wrapper: queryWrapper(queryClient) }
      )

      await waitFor(() => expect(result.current.data).toEqual({}))
      expect(get).toHaveBeenCalledWith('/api/data/usage_report', {
        params: {
          period: 'week',
          anchor: '2026-01-04',
          tz_offset: sentOffset,
        },
      })
    }
  )
})
