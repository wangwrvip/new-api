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
import { useQuery } from '@tanstack/react-query'

import { getGlmUsageBatch } from '../api'
import {
  channelsQueryKeys,
  chunkGlmChannelIds,
  mergeGlmUsageBatches,
} from '../lib'
import type { GlmUsageResult } from '../types'

const GLM_USAGE_STALE_TIME_MS = 60_000

function indexUsageById(
  data: Record<string, GlmUsageResult>
): Record<number, GlmUsageResult | undefined> {
  const byId: Record<number, GlmUsageResult | undefined> = {}
  for (const [id, result] of Object.entries(data)) {
    byId[Number(id)] = result
  }
  return byId
}

// Fetches GLM Coding Plan usage for a set of channels. The ids come from the
// current page's Zhipu channels; a page refresh costs one round trip per 50
// channels instead of one per row, and a batch failure only affects its ids.
export function useGlmUsage(channelIds: number[]) {
  const sortedIds = [...channelIds].sort((a, b) => a - b)

  return useQuery({
    queryKey: channelsQueryKeys.glmUsages(sortedIds),
    queryFn: async () => {
      const batches = chunkGlmChannelIds(sortedIds)
      const responses = await Promise.all(
        batches.map((ids) => getGlmUsageBatch(ids).catch(() => null))
      )
      return mergeGlmUsageBatches(responses, sortedIds)
    },
    enabled: sortedIds.length > 0,
    staleTime: GLM_USAGE_STALE_TIME_MS,
    select: (data) => indexUsageById(data),
  })
}
