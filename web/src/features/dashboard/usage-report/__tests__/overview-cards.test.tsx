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
import { describe, expect, it } from 'vitest'

import { OverviewCards } from '../components/overview-cards'
import type { UsageReportOverview } from '../types'

const baseOverview: UsageReportOverview = {
  requests: 120,
  prev_requests: 100,
  quota: 600000,
  prev_quota: 500000,
  tokens: 9000,
  prev_tokens: 0,
  active_users: 8,
  prev_active_users: 4,
}

describe('OverviewCards', () => {
  it('renders four metric cards with current values and period-over-period changes', () => {
    render(<OverviewCards overview={baseOverview} />)

    const requestsCard = screen.getByRole('listitem', { name: 'Requests' })
    expect(requestsCard).toHaveTextContent('120')
    expect(requestsCard).toHaveTextContent('+20.0%')

    const activeUsersCard = screen.getByRole('listitem', {
      name: 'Active users',
    })
    expect(activeUsersCard).toHaveTextContent('8')
    expect(activeUsersCard).toHaveTextContent('+100.0%')
  })

  it('shows placeholder dash when previous period has no baseline', () => {
    render(<OverviewCards overview={baseOverview} />)

    expect(screen.getByRole('listitem', { name: 'Tokens' })).toHaveTextContent(
      '—'
    )
  })
})
