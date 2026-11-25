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

import { getUsageReport } from '../api'
import type { UsageReportPeriod } from '../types'

export function useUsageReport(
  period: UsageReportPeriod,
  anchor: string | undefined
) {
  // 报告天边界跟随浏览器时区（getTimezoneOffset 对东八区返回 -480，取负得 +480）
  const tzOffsetMinutes = -new Date().getTimezoneOffset()
  return useQuery({
    queryKey: ['usage-report', period, anchor ?? '', tzOffsetMinutes],
    queryFn: () => getUsageReport({ period, anchor, tzOffsetMinutes }),
  })
}
