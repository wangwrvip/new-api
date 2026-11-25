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
import dayjs from '@/lib/dayjs'
import { getRollingDateRange, type TimeGranularity } from '@/lib/time'

import type { UserTimeRangePreset } from '../types'

/** Range presets for the User Analytics section: rolling day windows plus
 *  fixed calendar periods (this/last week, this/last month). */
export const USER_TIME_RANGE_PRESETS: ReadonlyArray<{
  id: UserTimeRangePreset
  labelKey: string
}> = [
  { id: '1d', labelKey: '1 Day' },
  { id: '7d', labelKey: '7 Days' },
  { id: '14d', labelKey: '14 Days' },
  { id: '29d', labelKey: '29 Days' },
  { id: 'this_week', labelKey: 'This week' },
  { id: 'last_week', labelKey: 'Last week' },
  { id: 'this_month', labelKey: 'Month' },
  { id: 'last_month', labelKey: 'Last month' },
]

/**
 * Resolve a User Analytics range preset to a concrete {start, end} window.
 * Calendar presets use the same dayjs boundaries as the shared date picker
 * (weeks start on Sunday) so the charts agree with the log filter.
 */
export function getUserRangeDates(
  preset: UserTimeRangePreset,
  now: Date = new Date()
): { start: Date; end: Date } {
  switch (preset) {
    case '1d':
      return getRollingDateRange(1, now)
    case '7d':
      return getRollingDateRange(7, now)
    case '14d':
      return getRollingDateRange(14, now)
    case '29d':
      return getRollingDateRange(29, now)
    case 'this_week': {
      const d = dayjs(now)
      return {
        start: d.startOf('week').toDate(),
        end: d.endOf('week').toDate(),
      }
    }
    case 'last_week': {
      const d = dayjs(now).subtract(1, 'week')
      return {
        start: d.startOf('week').toDate(),
        end: d.endOf('week').toDate(),
      }
    }
    case 'this_month': {
      const d = dayjs(now)
      return {
        start: d.startOf('month').toDate(),
        end: d.endOf('month').toDate(),
      }
    }
    case 'last_month': {
      const d = dayjs(now).subtract(1, 'month')
      return {
        start: d.startOf('month').toDate(),
        end: d.endOf('month').toDate(),
      }
    }
  }
}

/** Default range preset for a granularity. Replaces the day-count default used
 *  by the User Analytics section; the week granularity maps to the 29-day
 *  preset (the nearest tab) so the range tab stays highlighted. */
export function getDefaultUserRangePreset(
  granularity: TimeGranularity
): UserTimeRangePreset {
  switch (granularity) {
    case 'hour':
      return '1d'
    case 'day':
      return '7d'
    case 'week':
      return '29d'
  }
}
