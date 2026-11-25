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
import type { GlmUsageBatchResponse, GlmUsageResult } from '../types'

// The backend accepts at most 50 ids per request and silently drops the rest.
// Callers must split larger selections so every requested channel is covered.
export const GLM_USAGE_IDS_PER_REQUEST = 50

export function chunkGlmChannelIds(ids: number[]): number[][] {
  const batches: number[][] = []
  for (let i = 0; i < ids.length; i += GLM_USAGE_IDS_PER_REQUEST) {
    batches.push(ids.slice(i, i + GLM_USAGE_IDS_PER_REQUEST))
  }
  return batches
}

export function mergeGlmUsageBatches(
  batches: (GlmUsageBatchResponse | null)[],
  requestedIds: number[]
): Record<string, GlmUsageResult> {
  const merged: Record<string, GlmUsageResult> = {}
  for (const batch of batches) {
    if (batch?.success) {
      Object.assign(merged, batch.data ?? {})
    }
  }
  for (const id of requestedIds) {
    if (merged[id] === undefined) {
      merged[id] = { success: false, message: 'Failed to fetch usage' }
    }
  }
  return merged
}

export type GlmBadgeSegment = {
  labelKey: string
  remaining: number | null
}

// The two token windows GLM reports (5-hour rolling and weekly), shared by
// the balance-cell badge and the usage dialog. Windows missing from the
// upstream response surface as null so the UI can render a placeholder.
export function getGlmBadgeSegments(
  usage?: GlmUsageResult | null
): GlmBadgeSegment[] {
  return [
    { labelKey: '5h', remaining: usage?.five_h?.remaining_pct ?? null },
    { labelKey: 'Wk', remaining: usage?.weekly?.remaining_pct ?? null },
  ]
}

export function getWorstGlmRemainingPercent(
  usage?: GlmUsageResult | null
): number | null {
  const remainings = [
    usage?.five_h?.remaining_pct,
    usage?.weekly?.remaining_pct,
  ]
  const present = remainings.filter((value): value is number => value != null)
  if (present.length === 0) {
    return null
  }
  return Math.min(...present)
}

// Color tier for a remaining quota percentage, shared by the badge and the
// usage dialog: mirrors the glm-usage.py 95%/80%-used alarm thresholds.
export type GlmRemainingVariant = 'danger' | 'warning' | 'info'

export function getGlmRemainingVariant(remaining: number): GlmRemainingVariant {
  if (remaining <= 5) {
    return 'danger'
  }
  if (remaining <= 20) {
    return 'warning'
  }
  return 'info'
}
