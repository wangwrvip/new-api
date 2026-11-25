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
/**
 * 环比变化百分比：(current - previous) / previous * 100。
 * 上期为 0 或负数时返回 null（界面显示 "—"）。
 */
export function percentChange(
  current: number,
  previous: number
): number | null {
  if (previous <= 0) return null
  return ((current - previous) / previous) * 100
}

/** 格式化为 "+12.3%" / "-8.0%"；null 返回 null，由调用方渲染占位符 */
export function formatPercentChange(value: number | null): string | null {
  if (value == null) return null
  const sign = value > 0 ? '+' : ''
  return `${sign}${value.toFixed(1)}%`
}
