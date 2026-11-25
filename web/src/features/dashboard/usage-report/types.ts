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
export type UsageReportPeriod = 'week' | 'month'

export interface UsageReportPeriodInfo {
  label: string
  start_date: string
  end_date: string
}

export interface UsageReportOverview {
  requests: number
  prev_requests: number
  quota: number
  prev_quota: number
  tokens: number
  prev_tokens: number
  active_users: number
  prev_active_users: number
}

export interface UsageReportDaily {
  date: string
  requests: number
  quota: number
  active_users: number
}

export interface UsageReportUser {
  user_id: number
  username: string
  /** 主库显示名；未设置或已删除用户为空，展示时回退 username */
  display_name?: string
  /** 主库部门；已从 users 表删除（软删）的用户为空 */
  department?: string
  requests: number
  prev_requests: number
  active_days: number
  prev_active_days: number
  quota: number
  prev_quota: number
  tokens: number
  prev_tokens: number
  /** 归一化 token 维度（本期）：非缓存输入 / 输出 / 缓存读+写合计；旧日志为 0 */
  input_tokens?: number
  output_tokens?: number
  cache_tokens?: number
  /** 缓存命中的读分量：命中率 = cache_read_tokens / (input_tokens + cache_tokens)；旧日志为 0 */
  cache_read_tokens?: number
}

/** 单个部门的活跃统计。active_rate 为本期活跃用户 / 总用户（0..1）。 */
export interface UsageReportDepartment {
  department: string
  total_users: number
  active_users: number
  prev_active_users: number
  active_rate: number
}

export interface UsageReport {
  period: UsageReportPeriod
  current: UsageReportPeriodInfo
  previous: UsageReportPeriodInfo
  overview: UsageReportOverview
  daily: UsageReportDaily[]
  departments: UsageReportDepartment[]
  users: UsageReportUser[]
}
