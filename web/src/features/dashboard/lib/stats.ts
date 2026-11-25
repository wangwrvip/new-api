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
import type { QuotaDataItem } from '@/features/dashboard/types'

/**
 * Safe division: handles NaN and Infinity cases
 */
export function safeDivide(
  value: number,
  divisor: number,
  precision: number = 3
): number {
  const result = value / divisor
  if (Number.isNaN(result) || !Number.isFinite(result)) return 0
  const factor = Math.pow(10, precision)
  return Math.round(result * factor) / factor
}

/**
 * Calculate aggregated statistics from quota data
 */
export function calculateDashboardStats(data: QuotaDataItem[]) {
  return data.reduce(
    (acc, item) => {
      const cacheRead = item.cache_read_tokens ?? 0
      const cacheCreation = item.cache_creation_tokens ?? 0
      return {
        totalQuota: acc.totalQuota + (Number(item.quota) || 0),
        totalCount: acc.totalCount + (Number(item.count) || 0),
        totalTokens: acc.totalTokens + (Number(item.token_used) || 0),
        totalCacheReadTokens:
          acc.totalCacheReadTokens +
          (Number.isFinite(cacheRead) ? Math.max(0, cacheRead) : 0),
        totalCacheCreationTokens:
          acc.totalCacheCreationTokens +
          (Number.isFinite(cacheCreation) ? Math.max(0, cacheCreation) : 0),
        inputTokens: acc.inputTokens + (Number(item.input_tokens) || 0),
        outputTokens: acc.outputTokens + (Number(item.output_tokens) || 0),
        cacheTokens: acc.cacheTokens + (Number(item.cache_tokens) || 0),
        cacheReadTokens:
          acc.cacheReadTokens +
          (Number.isFinite(cacheRead) ? Math.max(0, cacheRead) : 0),
      }
    },
    {
      totalQuota: 0,
      totalCount: 0,
      totalTokens: 0,
      totalCacheReadTokens: 0,
      totalCacheCreationTokens: 0,
      inputTokens: 0,
      outputTokens: 0,
      cacheTokens: 0,
      cacheReadTokens: 0,
    }
  )
}

/**
 * Cache hit rate: cache read tokens over the input-side total
 * (non-cached input + cache read + cache write). Returns a 0..1 fraction;
 * 0 when there are no input-side tokens (legacy rows show 0).
 */
export function cacheHitRate(
  cacheReadTokens: number,
  inputTokens: number,
  cacheTokens: number
): number {
  const inputSide = (Number(inputTokens) || 0) + (Number(cacheTokens) || 0)
  if (inputSide <= 0) return 0
  return Math.min(1, Math.max(0, (Number(cacheReadTokens) || 0) / inputSide))
}
