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

import { formatChartTime } from '../time'

// 2026-08-30 is a Sunday: the first day of the calendar week (Sunday start,
// matching the shared date picker).
const WEEK_BASE = dayjs('2026-08-30T12:00:00')

describe('formatChartTime week granularity', () => {
  test('all days in the same calendar week share one label', () => {
    const sunday = WEEK_BASE.unix()
    const wednesday = WEEK_BASE.add(3, 'day').unix()
    const saturday = WEEK_BASE.add(6, 'day').unix()
    const label = '08-30 - 09-05'
    expect(formatChartTime(sunday, 'week')).toBe(label)
    expect(formatChartTime(wednesday, 'week')).toBe(label)
    expect(formatChartTime(saturday, 'week')).toBe(label)
  })

  test('the next calendar week gets its own label', () => {
    const nextSunday = WEEK_BASE.add(7, 'day').unix()
    expect(formatChartTime(nextSunday, 'week')).toBe('09-06 - 09-12')
  })

  test('day and hour granularities are unchanged', () => {
    const wednesday = WEEK_BASE.add(3, 'day')
    expect(formatChartTime(wednesday.unix(), 'day')).toBe('09-02')
    expect(formatChartTime(wednesday.unix(), 'hour')).toBe('09-02 12:00')
  })
})
