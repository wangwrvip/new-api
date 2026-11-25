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
import { render, screen, waitFor } from '@testing-library/react'
import { describe, expect, it, vi } from 'vitest'

import { getUserQuotaDates } from '@/features/dashboard/api'

import { LogStatCards } from '../log-stat-cards'

vi.mock('@/features/dashboard/api', () => ({
  getUserQuotaDates: vi.fn(),
}))

const mockedGetUserQuotaDates = vi.mocked(getUserQuotaDates)

describe('LogStatCards token dimensions', () => {
  it('shows the input/output/cache breakdown on the Total Tokens card', async () => {
    mockedGetUserQuotaDates.mockResolvedValue({
      data: [
        {
          created_at: 1700000000,
          token_used: 1200,
          input_tokens: 850,
          output_tokens: 200,
          cache_tokens: 150,
          quota: 50,
          count: 1,
        },
      ],
    } as never)

    render(<LogStatCards />)

    await waitFor(() => {
      expect(screen.getByText('Total Tokens')).toBeInTheDocument()
    })

    const tokensCard = screen
      .getByText('Total Tokens')
      .closest('div.grid > div')
    expect(tokensCard).not.toBeNull()
    await waitFor(() => {
      expect(tokensCard).toHaveTextContent('Input 850')
      expect(tokensCard).toHaveTextContent('Output 200')
      expect(tokensCard).toHaveTextContent('Cache 150')
    })
  })

  it('shows the cache hit rate card with hit tokens when cache data exists', async () => {
    mockedGetUserQuotaDates.mockResolvedValue({
      data: [
        {
          created_at: 1700000000,
          token_used: 1200,
          input_tokens: 850,
          output_tokens: 200,
          cache_tokens: 150,
          cache_read_tokens: 100,
          quota: 50,
          count: 1,
        },
      ],
    } as never)

    render(<LogStatCards />)

    await waitFor(() => {
      expect(screen.getByText('Cache Hit Rate')).toBeInTheDocument()
    })

    const hitCard = screen.getByText('Cache Hit Rate').closest('div.grid > div')
    expect(hitCard).not.toBeNull()
    await waitFor(() => {
      // 100 / (850 + 150) = 10%
      expect(hitCard).toHaveTextContent('10.0%')
      expect(hitCard).toHaveTextContent('Cache hit tokens 100')
    })
  })

  it('omits the breakdown when the range has no dimension data', async () => {
    mockedGetUserQuotaDates.mockResolvedValue({
      data: [{ created_at: 1700000000, token_used: 300, quota: 10, count: 2 }],
    } as never)

    render(<LogStatCards />)

    await waitFor(() => {
      expect(screen.getByText('Total Tokens')).toBeInTheDocument()
    })

    const tokensCard = screen
      .getByText('Total Tokens')
      .closest('div.grid > div')
    await waitFor(() => {
      expect(tokensCard).toHaveTextContent('300')
    })
    expect(tokensCard).not.toHaveTextContent('Input')
  })
})
