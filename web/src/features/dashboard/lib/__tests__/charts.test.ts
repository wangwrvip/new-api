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
import { describe, expect, it, test } from 'vitest'

import type { QuotaDataItem } from '@/features/dashboard/types'
import type { TimeGranularity } from '@/lib/time'

import { processChartData, processUserChartData } from '../charts'

const cases: Array<{
  granularity: TimeGranularity
  start: string
  interval: number
  labels: string[]
}> = [
  {
    granularity: 'hour',
    start: '2025-12-31T20:00:00',
    interval: 3600,
    labels: [
      '12-31 20:00',
      '12-31 21:00',
      '12-31 22:00',
      '12-31 23:00',
      '01-01 00:00',
      '01-01 01:00',
      '01-01 02:00',
      '01-01 03:00',
    ],
  },
  {
    granularity: 'day',
    start: '2025-12-27T12:00:00',
    interval: 86400,
    labels: [
      '12-27',
      '12-28',
      '12-29',
      '12-30',
      '12-31',
      '01-01',
      '01-02',
      '01-03',
    ],
  },
  {
    granularity: 'week',
    start: '2025-11-29T12:00:00',
    interval: 604800,
    labels: [
      '11-23 - 11-29',
      '11-30 - 12-06',
      '12-07 - 12-13',
      '12-14 - 12-20',
      '12-21 - 12-27',
      '12-28 - 01-03',
      '01-04 - 01-10',
      '01-11 - 01-17',
    ],
  },
]

function unorderedUsage(start: string, interval: number): QuotaDataItem[] {
  const first = new Date(start).getTime() / 1000
  const rows = Array.from({ length: 8 }, (_, i) => ({
    created_at: first + i * interval,
    model_name: 'model-a',
    username: 'alice',
    quota: (i + 1) * 500000,
    count: i + 1,
  }))
  // Same hour, or a second hourly record within the same day/week label.
  const duplicate = {
    ...rows[0],
    created_at: first + (interval === 3600 ? 0 : 3600),
    quota: 5000000,
    count: 10,
  }
  return [
    rows[7],
    rows[2],
    duplicate,
    rows[5],
    rows[0],
    rows[6],
    rows[1],
    rows[4],
    rows[3],
  ]
}

describe.each(cases)('dashboard $granularity chart chronology', (scenario) => {
  test('orders model buckets across New Year while aggregating repeated labels', () => {
    const data = unorderedUsage(scenario.start, scenario.interval)
    const original = structuredClone(data)
    const result = processChartData(data, scenario.granularity)

    for (const key of ['spec_line', 'spec_area', 'spec_model_line'] as const) {
      const values: Array<{ Time: string; rawQuota: number; Count: number }> =
        result[key].data[0].values
      expect(values.map((row) => row.Time)).toEqual(scenario.labels)
      expect(
        values.map((row) =>
          key === 'spec_model_line' ? row.Count : row.rawQuota / 500000
        )
      ).toEqual([11, 2, 3, 4, 5, 6, 7, 8])
    }
    expect(data).toEqual(original)
  })

  test('orders user buckets across New Year while aggregating repeated labels', () => {
    const data = unorderedUsage(scenario.start, scenario.interval)
    const original = structuredClone(data)
    const result = processUserChartData(data, scenario.granularity)
    const values: Array<{ Time: string; rawQuota: number }> =
      result.spec_user_trend.data[0].values

    expect(values.map((row) => row.Time)).toEqual(scenario.labels)
    expect(values.map((row) => row.rawQuota / 500000)).toEqual([
      11, 2, 3, 4, 5, 6, 7, 8,
    ])
    expect(data).toEqual(original)
  })

  test('keeps the existing seven padded model buckets in chronological order', () => {
    const data = [unorderedUsage(scenario.start, scenario.interval)[0]]
    const result = processChartData(data, scenario.granularity)

    for (const key of ['spec_line', 'spec_area', 'spec_model_line'] as const) {
      const values: Array<{ Time: string; rawQuota: number; Count: number }> =
        result[key].data[0].values
      expect(values.map((row) => row.Time)).toEqual(scenario.labels.slice(1))
      expect(
        values.map((row) =>
          key === 'spec_model_line' ? row.Count : row.rawQuota / 500000
        )
      ).toEqual([0, 0, 0, 0, 0, 0, 8])
    }
  })
})

interface CacheRateDatum {
  Model?: string
  User?: string
  Rate: number
  CacheRead: number
  InputSide: number
}

function cacheRateValues(spec: unknown): CacheRateDatum[] {
  const values = (spec as { data: Array<{ values: unknown[] }> }).data[0]
    .values as CacheRateDatum[]
  return values
}

describe('processChartData cache hit ranking', () => {
  it('ranks models by cache hit rate and excludes models without input-side tokens', () => {
    const data: QuotaDataItem[] = [
      {
        created_at: 1700000000,
        model_name: 'model-low',
        input_tokens: 800,
        output_tokens: 100,
        cache_tokens: 200,
        cache_read_tokens: 150,
        token_used: 1100,
        count: 2,
        quota: 10,
      },
      {
        created_at: 1700000000,
        model_name: 'model-high',
        input_tokens: 100,
        output_tokens: 50,
        cache_tokens: 900,
        cache_read_tokens: 850,
        token_used: 1050,
        count: 3,
        quota: 20,
      },
      {
        created_at: 1700000000,
        // 旧数据：无维度列，输入侧为 0，不应进入命中率排行
        model_name: 'model-legacy',
        token_used: 500,
        count: 5,
        quota: 30,
      },
    ]

    const result = processChartData(data, 'hour')
    const values = cacheRateValues(result.spec_cache_rate_bar)

    expect(values.map((v) => v.Model)).toEqual(['model-high', 'model-low'])
    // model-high: 850 / (100 + 900) = 85
    expect(values[0].Rate).toBe(85)
    expect(values[0].CacheRead).toBe(850)
    expect(values[0].InputSide).toBe(1000)
    // model-low: 150 / (800 + 200) = 15
    expect(values[1].Rate).toBe(15)
  })

  it('emits an empty cache ranking when there is no data', () => {
    const result = processChartData([], 'hour')

    expect(cacheRateValues(result.spec_cache_rate_bar)).toEqual([])
  })
})

describe('processUserChartData cache hit rate', () => {
  it('ranks users by hit rate within the limit', () => {
    const data: QuotaDataItem[] = [
      {
        created_at: 1700000000,
        username: 'alice',
        input_tokens: 100,
        output_tokens: 50,
        cache_tokens: 900,
        cache_read_tokens: 850,
        token_used: 1050,
        count: 3,
        quota: 20,
      },
      {
        created_at: 1700000000,
        username: 'bob',
        input_tokens: 800,
        output_tokens: 100,
        cache_tokens: 200,
        cache_read_tokens: 150,
        token_used: 1100,
        count: 2,
        quota: 10,
      },
    ]

    const result = processUserChartData(data, 'hour', undefined, 10)
    const values = cacheRateValues(result.spec_user_cache_rate)

    expect(values.map((v) => v.User)).toEqual(['alice', 'bob'])
    expect(values[0].Rate).toBe(85)
    expect(values[1].Rate).toBe(15)
  })

  it('emits an empty user cache ranking when there is no data', () => {
    const result = processUserChartData([], 'hour')

    expect(cacheRateValues(result.spec_user_cache_rate)).toEqual([])
  })
})
