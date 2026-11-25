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
import { describe, expect, test } from 'vitest'

import dayjs from '@/lib/dayjs'

import type { UserTimeRangePreset } from '../types'
import {
  USER_TIME_RANGE_PRESETS,
  getUserRangeDates,
  getDefaultUserRangePreset,
} from './user-range'

// A fixed Wednesday (2026-09-02 15:30 local) so calendar-boundary tests are
// deterministic regardless of when the suite runs.
const FIXED_NOW = new Date(2026, 8, 2, 15, 30, 0)

const DAY_MS = 86_400_000

describe('getUserRangeDates rolling presets', () => {
  test.each<[UserTimeRangePreset, number]>([
    ['1d', 1],
    ['7d', 7],
    ['14d', 14],
    ['29d', 29],
  ])('%s spans a %i-day window ending at now', (preset, days) => {
    const { start, end } = getUserRangeDates(preset, FIXED_NOW)
    expect(start.getTime()).toBe(FIXED_NOW.getTime() - days * DAY_MS)
    expect(end.getTime()).toBe(FIXED_NOW.getTime())
  })
})

describe('getUserRangeDates calendar presets', () => {
  test('this_week is the calendar week containing now', () => {
    const { start, end } = getUserRangeDates('this_week', FIXED_NOW)
    const weekStart = dayjs(FIXED_NOW).startOf('week')
    expect(start.toISOString()).toBe(weekStart.toDate().toISOString())
    expect(end.toISOString()).toBe(
      weekStart.endOf('week').toDate().toISOString()
    )
    expect(end.getTime() - start.getTime()).toBe(6 * DAY_MS + DAY_MS - 1)
  })

  test('last_week is the calendar week before this_week', () => {
    const { start, end } = getUserRangeDates('last_week', FIXED_NOW)
    const thisWeek = getUserRangeDates('this_week', FIXED_NOW)
    expect(end.getTime()).toBe(thisWeek.start.getTime() - 1)
    expect(start.getTime()).toBe(thisWeek.start.getTime() - 7 * DAY_MS)
  })

  test('this_month spans the calendar month containing now', () => {
    const { start, end } = getUserRangeDates('this_month', FIXED_NOW)
    expect(start.toISOString()).toBe(
      dayjs(FIXED_NOW).startOf('month').toDate().toISOString()
    )
    expect(end.toISOString()).toBe(
      dayjs(FIXED_NOW).endOf('month').toDate().toISOString()
    )
  })

  test('last_month is the calendar month before this_month', () => {
    const { start, end } = getUserRangeDates('last_month', FIXED_NOW)
    const thisMonth = getUserRangeDates('this_month', FIXED_NOW)
    expect(end.getTime()).toBe(thisMonth.start.getTime() - 1)
    expect(start.getTime()).toBe(
      dayjs(FIXED_NOW).subtract(1, 'month').startOf('month').toDate().getTime()
    )
  })
})

describe('getDefaultUserRangePreset', () => {
  test('maps each granularity to a matching range preset', () => {
    expect(getDefaultUserRangePreset('hour')).toBe('1d')
    expect(getDefaultUserRangePreset('day')).toBe('7d')
    expect(getDefaultUserRangePreset('week')).toBe('29d')
  })
})

describe('USER_TIME_RANGE_PRESETS', () => {
  test('exposes the rolling plus calendar presets in order', () => {
    expect(USER_TIME_RANGE_PRESETS.map((p) => p.id)).toEqual([
      '1d',
      '7d',
      '14d',
      '29d',
      'this_week',
      'last_week',
      'this_month',
      'last_month',
    ])
  })
})
