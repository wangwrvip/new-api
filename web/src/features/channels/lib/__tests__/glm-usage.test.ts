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
import { renderHook, waitFor } from '@testing-library/react'
import { createElement, type ReactNode } from 'react'
import { afterEach, beforeEach, describe, expect, test, vi } from 'vitest'

import { getGlmUsageBatch } from '../../api'
import { useGlmUsage } from '../../hooks/use-glm-usage'
import type { GlmUsageBatchResponse, GlmUsageResult } from '../../types'
import {
  chunkGlmChannelIds,
  getGlmBadgeSegments,
  getGlmRemainingVariant,
  getWorstGlmRemainingPercent,
  mergeGlmUsageBatches,
} from '../glm-usage'

// Mock the network boundary: the hook must send one request per 50 ids, not
// simply chunk a pure helper that no caller is forced to use.
vi.mock('../../api', async (importOriginal) => {
  const actual = await importOriginal<typeof import('../../api')>()
  return { ...actual, getGlmUsageBatch: vi.fn() }
})

const batchRequest = vi.mocked(getGlmUsageBatch)

function successfulBatch(ids: number[]): GlmUsageBatchResponse {
  return {
    success: true,
    data: Object.fromEntries(ids.map((id) => [String(id), fullUsage])),
  }
}

const failedUsage = {
  success: false,
  message: 'Failed to fetch usage',
} as const

let queryClient: QueryClient

function wrapper(props: { children: ReactNode }) {
  return createElement(
    QueryClientProvider,
    { client: queryClient },
    props.children
  )
}

beforeEach(() => {
  queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })
  batchRequest.mockReset()
})

afterEach(() => {
  queryClient.clear()
})

const fullUsage: GlmUsageResult = {
  success: true,
  level: 'MAX',
  five_h: { used_pct: 7, remaining_pct: 93, next_reset_ms: 1789009045416 },
  weekly: { used_pct: 71, remaining_pct: 29, next_reset_ms: 1789374107996 },
}

describe('GLM usage batching', () => {
  test('splits a full page of 100 channel ids into two 50-id requests', () => {
    const ids = Array.from({ length: 100 }, (_, index) => index + 1)

    const batches = chunkGlmChannelIds(ids)

    expect(batches).toHaveLength(2)
    expect(batches[0]).toHaveLength(50)
    expect(batches[1]).toHaveLength(50)
    expect(batches.flat()).toEqual(ids)
  })

  test('keeps a short selection in a single request', () => {
    expect(chunkGlmChannelIds([9, 4])).toEqual([[9, 4]])
  })

  test('merges every batch and marks ids from a failed batch as failed', () => {
    const first: GlmUsageBatchResponse = {
      success: true,
      data: { '1': fullUsage },
    }
    const failed: GlmUsageBatchResponse = {
      success: false,
      message: 'Failed to fetch usage',
    }

    const merged = mergeGlmUsageBatches([first, failed], [1, 2])

    expect(merged['1']).toEqual(fullUsage)
    expect(merged['2']).toEqual({
      success: false,
      message: 'Failed to fetch usage',
    })
  })
})

describe('useGlmUsage request batching', () => {
  test('sends 100 ids in exactly two 50-id requests and returns every id once', async () => {
    const ids = Array.from({ length: 100 }, (_, index) => index + 1)
    batchRequest.mockImplementation(async (requested) =>
      successfulBatch(requested)
    )

    const { result } = renderHook(() => useGlmUsage(ids), { wrapper })

    await waitFor(() => expect(result.current.data).toHaveProperty('100'))
    const batches = batchRequest.mock.calls.map(([requested]) => requested)
    expect(batchRequest).toHaveBeenCalledTimes(2)
    expect(batches).toEqual([ids.slice(0, 50), ids.slice(50, 100)])
    expect(batches.flat()).toEqual(ids)
    expect(new Set(batches.flat()).size).toBe(100)
    for (const id of ids) {
      expect(result.current.data?.[id]).toEqual(fullUsage)
    }
  })

  test('marks only the ids of a failed batch as failed and keeps the other batch data', async () => {
    const ids = Array.from({ length: 100 }, (_, index) => index + 1)
    batchRequest
      .mockResolvedValueOnce(successfulBatch(ids.slice(0, 50)))
      .mockRejectedValueOnce(new Error('HTTP 500'))

    const { result } = renderHook(() => useGlmUsage(ids), { wrapper })

    await waitFor(() => expect(result.current.data).toHaveProperty('100'))
    const batches = batchRequest.mock.calls.map(([requested]) => requested)
    expect(batches).toEqual([ids.slice(0, 50), ids.slice(50, 100)])
    for (const id of ids.slice(0, 50)) {
      expect(result.current.data?.[id]).toEqual(fullUsage)
    }
    for (const id of ids.slice(50, 100)) {
      expect(result.current.data?.[id]).toEqual(failedUsage)
    }
  })

  test.each([
    { count: 37, batches: [Array.from({ length: 37 }, (_, i) => i + 1)] },
    {
      count: 101,
      batches: [
        Array.from({ length: 50 }, (_, i) => i + 1),
        Array.from({ length: 50 }, (_, i) => i + 51),
        [101],
      ],
    },
  ])(
    'splits $count ids into the expected batches without losing ids',
    async ({ count, batches }) => {
      const ids = Array.from({ length: count }, (_, index) => index + 1)
      batchRequest.mockImplementation(async (requested) =>
        successfulBatch(requested)
      )

      const { result } = renderHook(() => useGlmUsage(ids), { wrapper })

      await waitFor(() =>
        expect(result.current.data).toHaveProperty(String(count))
      )
      const requests = batchRequest.mock.calls.map(([requested]) => requested)
      expect(requests).toEqual(batches)
      expect(requests.flat()).toEqual(ids)
      for (const id of ids) {
        expect(result.current.data?.[id]).toEqual(fullUsage)
      }
    }
  )
})

describe('getGlmBadgeSegments', () => {
  test('returns both window remainings when usage has five-hour and weekly slots', () => {
    expect(getGlmBadgeSegments(fullUsage)).toEqual([
      { labelKey: '5h', remaining: 93 },
      { labelKey: 'Wk', remaining: 29 },
    ])
  })

  test('returns null remaining for windows missing from the upstream response', () => {
    const partial: GlmUsageResult = {
      success: true,
      five_h: { used_pct: 7, remaining_pct: 93, next_reset_ms: 1 },
    }
    expect(getGlmBadgeSegments(partial)).toEqual([
      { labelKey: '5h', remaining: 93 },
      { labelKey: 'Wk', remaining: null },
    ])
  })

  test('returns null remainings for a failed usage result', () => {
    const failed: GlmUsageResult = {
      success: false,
      message: 'upstream status: 401',
    }
    expect(getGlmBadgeSegments(failed)).toEqual([
      { labelKey: '5h', remaining: null },
      { labelKey: 'Wk', remaining: null },
    ])
  })

  test('returns null remainings when no usage has been fetched yet', () => {
    expect(getGlmBadgeSegments(undefined)).toEqual([
      { labelKey: '5h', remaining: null },
      { labelKey: 'Wk', remaining: null },
    ])
  })
})

describe('getWorstGlmRemainingPercent', () => {
  test('returns the smallest remaining across present windows', () => {
    expect(getWorstGlmRemainingPercent(fullUsage)).toBe(29)
  })

  test('returns null when neither window is present', () => {
    expect(getWorstGlmRemainingPercent({ success: false })).toBeNull()
  })
})

describe('getGlmRemainingVariant', () => {
  test('marks nearly exhausted windows as danger', () => {
    expect(getGlmRemainingVariant(0)).toBe('danger')
    expect(getGlmRemainingVariant(5)).toBe('danger')
  })

  test('marks low but not exhausted windows as warning', () => {
    expect(getGlmRemainingVariant(6)).toBe('warning')
    expect(getGlmRemainingVariant(20)).toBe('warning')
  })

  test('marks healthy windows as info', () => {
    expect(getGlmRemainingVariant(21)).toBe('info')
    expect(getGlmRemainingVariant(100)).toBe('info')
  })
})
