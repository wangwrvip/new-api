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
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, test, vi } from 'vitest'

import { formatTimestampToDate } from '@/lib/format'

import type { GlmUsageResult } from '../../../types'
import { GlmUsageDialog } from '../glm-usage-dialog'

const liveStyleUsage: GlmUsageResult = {
  success: true,
  level: 'MAX',
  five_h: { used_pct: 7, remaining_pct: 93, next_reset_ms: 1789009045416 },
  weekly: { used_pct: 71, remaining_pct: 29, next_reset_ms: 1789374107996 },
}

function renderDialog(props: Partial<Parameters<typeof GlmUsageDialog>[0]>) {
  return render(
    <GlmUsageDialog
      open
      onOpenChange={vi.fn()}
      channelName='GLM-Pro'
      channelId={7}
      response={null}
      {...props}
    />
  )
}

describe('GlmUsageDialog', () => {
  test('renders both windows with remaining, used and reset times plus the plan level', () => {
    renderDialog({ response: liveStyleUsage })

    expect(screen.getByText('5-Hour Window')).toBeInTheDocument()
    expect(screen.getByText('Weekly Window')).toBeInTheDocument()
    expect(screen.getByText('93%')).toBeInTheDocument()
    expect(screen.getByText('29%')).toBeInTheDocument()
    expect(screen.getByText('Used 7%')).toBeInTheDocument()
    expect(screen.getByText('Used 71%')).toBeInTheDocument()
    expect(
      screen.getByText(
        formatTimestampToDate(
          liveStyleUsage.five_h?.next_reset_ms ?? 0,
          'milliseconds'
        )
      )
    ).toBeInTheDocument()
    expect(
      screen.getByText(
        formatTimestampToDate(
          liveStyleUsage.weekly?.next_reset_ms ?? 0,
          'milliseconds'
        )
      )
    ).toBeInTheDocument()
    expect(screen.getByText('Plan MAX')).toBeInTheDocument()
    expect(screen.getByText('GLM-Pro (#7)')).toBeInTheDocument()
  })

  test('shows the upstream failure message instead of window data when the result failed', () => {
    renderDialog({
      response: { success: false, message: 'upstream status: 401' },
    })

    expect(screen.getByText('upstream status: 401')).toBeInTheDocument()
    expect(screen.queryByText('93%')).not.toBeInTheDocument()
  })

  test('shows the empty state for a missing response and refresh triggers the callback', async () => {
    const onRefresh = vi.fn()
    const user = userEvent.setup()
    renderDialog({ response: null, onRefresh })

    expect(screen.getByText('No usage data')).toBeInTheDocument()

    await user.click(screen.getByRole('button', { name: 'Refresh' }))
    expect(onRefresh).toHaveBeenCalledTimes(1)
  })
})
