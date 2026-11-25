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
import { describe, expect, it } from 'vitest'

import type { QuotaDataItem } from '../../types'
import { cacheHitRate, calculateDashboardStats } from '../stats'

describe('calculateDashboardStats', () => {
  it('sums normalized token dimensions alongside the unified total', () => {
    const data: QuotaDataItem[] = [
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
      {
        created_at: 1700003600,
        token_used: 120,
        input_tokens: 40,
        output_tokens: 10,
        cache_tokens: 70,
        cache_read_tokens: 50,
        quota: 5,
        count: 1,
      },
    ]

    const stats = calculateDashboardStats(data)

    expect(stats.totalTokens).toBe(1320)
    expect(stats.inputTokens).toBe(890)
    expect(stats.outputTokens).toBe(210)
    expect(stats.cacheTokens).toBe(220)
    expect(stats.cacheReadTokens).toBe(150)
    expect(stats.totalCacheReadTokens).toBe(150)
    expect(stats.totalCacheCreationTokens).toBe(0)
  })

  it('keeps dimensions at zero when legacy rows carry no dimension columns', () => {
    const data: QuotaDataItem[] = [
      { created_at: 1700000000, token_used: 300, quota: 10, count: 2 },
    ]

    const stats = calculateDashboardStats(data)

    expect(stats.totalTokens).toBe(300)
    expect(stats.inputTokens).toBe(0)
    expect(stats.outputTokens).toBe(0)
    expect(stats.cacheTokens).toBe(0)
    expect(stats.cacheReadTokens).toBe(0)
  })
})

describe('cacheHitRate', () => {
  it('computes cache read share of input-side tokens (read + write + non-cached)', () => {
    // 108 / (880 + 200) = 0.1
    expect(cacheHitRate(108, 880, 200)).toBe(0.1)
  })

  it('returns 0 when the input side has no tokens', () => {
    expect(cacheHitRate(0, 0, 0)).toBe(0)
  })

  it('keeps legacy rows (read 0) at rate 0 even with cache totals', () => {
    expect(cacheHitRate(0, 500, 300)).toBe(0)
  })
})
