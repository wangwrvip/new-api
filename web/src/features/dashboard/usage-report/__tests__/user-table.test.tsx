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

import { UserTable } from '../components/user-table'
import type { UsageReportUser } from '../types'

const users: UsageReportUser[] = [
  {
    user_id: 1,
    username: 'alice',
    requests: 50,
    prev_requests: 40,
    active_days: 6,
    prev_active_days: 5,
    quota: 600000,
    prev_quota: 500000,
    tokens: 900,
    prev_tokens: 800,
  },
  {
    user_id: 2,
    username: 'bob',
    requests: 10,
    prev_requests: 30,
    active_days: 2,
    prev_active_days: 5,
    quota: 100000,
    prev_quota: 300000,
    tokens: 100,
    prev_tokens: 300,
  },
]

describe('UserTable', () => {
  it('renders one row per user with requests and active days', () => {
    render(<UserTable users={users} />)

    expect(screen.getByRole('cell', { name: 'alice' })).toBeInTheDocument()
    expect(screen.getByRole('cell', { name: 'bob' })).toBeInTheDocument()
    expect(screen.getByText('50')).toBeInTheDocument()
    expect(screen.getByText('6')).toBeInTheDocument()
  })

  it('renders period-over-period change for each user', () => {
    render(<UserTable users={users} />)

    expect(screen.getByText(/↑ \+25\.0%/)).toBeInTheDocument()
    expect(screen.getByText(/↓ -66\.7%/)).toBeInTheDocument()
  })

  it('renders empty state when no users', () => {
    render(<UserTable users={[]} />)

    expect(
      screen.getByText('No usage records in this period')
    ).toBeInTheDocument()
  })
})

describe('UserTable token dimensions', () => {
  it('renders input/output/cache breakdown under the total tokens per user', () => {
    const dimUsers: UsageReportUser[] = [
      {
        ...users[0],
        tokens: 900,
        input_tokens: 500,
        output_tokens: 300,
        cache_tokens: 100,
      },
    ]

    render(<UserTable users={dimUsers} />)

    const tokensCell = screen.getByRole('cell', { name: /^900/ })
    expect(tokensCell).toHaveTextContent('Input 500')
    expect(tokensCell).toHaveTextContent('Output 300')
    expect(tokensCell).toHaveTextContent('Cache 100')
  })

  it('appends the cache hit rate to the breakdown when cache read tokens exist', () => {
    const dimUsers: UsageReportUser[] = [
      {
        ...users[0],
        tokens: 900,
        input_tokens: 500,
        output_tokens: 300,
        cache_tokens: 100,
        cache_read_tokens: 80,
      },
    ]

    render(<UserTable users={dimUsers} />)

    // 80 / (500 + 100) = 13.3%
    const tokensCell = screen.getByRole('cell', { name: /^900/ })
    expect(tokensCell).toHaveTextContent('13.3%')
  })

  it('omits the hit rate when cache read tokens are missing (legacy rows)', () => {
    const dimUsers: UsageReportUser[] = [
      {
        ...users[0],
        tokens: 900,
        input_tokens: 500,
        output_tokens: 300,
        cache_tokens: 100,
      },
    ]

    render(<UserTable users={dimUsers} />)

    const tokensCell = screen.getByRole('cell', { name: /^900/ })
    expect(tokensCell).not.toHaveTextContent('%')
  })

  it('hides the breakdown line when a user has no dimension data', () => {
    render(<UserTable users={users} />)

    expect(screen.getByRole('cell', { name: '900' })).not.toHaveTextContent(
      'Input'
    )
  })
})
