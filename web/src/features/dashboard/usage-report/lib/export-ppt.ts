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
import i18n, { type TFunction } from 'i18next'
import pptxgenjs from 'pptxgenjs'

import { toIntlLocale } from '@/i18n/languages'
import { quotaUnitsToDollars } from '@/lib/format'

import type { UsageReport } from '../types'
import { formatPercentChange, percentChange } from './stats'

// 每张用户页最多容纳的用户行数（不含表头）
const USERS_PER_SLIDE = 10
const TOP_PIE_USERS = 10

export interface ReportSlides {
  kind: 'title' | 'overview' | 'departments' | 'trend' | 'users' | 'cost-share'
  title: string
  /** overview/departments/users 页的表格行 */
  rows?: string[][]
  /** trend 页横轴与两组数值；cost-share 页为 labels/values */
  labels?: string[]
  requestsValues?: number[]
  costValues?: number[]
  values?: number[]
}

function changeText(current: number, previous: number): string {
  const formatted = formatPercentChange(percentChange(current, previous))
  return formatted ?? '—'
}

// userSlides 把全部用户按 USERS_PER_SLIDE 分页，每页生成一张 users 页；
// 无用户时仍生成一页（仅表头），标题带页码 (x/y)。
function userSlides(users: UsageReport['users'], t: TFunction): ReportSlides[] {
  const header = [
    t('Department'),
    t('User'),
    t('Requests'),
    t('Change'),
    t('Active days'),
    t('Cost'),
    t('Change'),
  ]
  const chunks: UsageReport['users'][] = []
  for (let i = 0; i < users.length; i += USERS_PER_SLIDE) {
    chunks.push(users.slice(i, i + USERS_PER_SLIDE))
  }
  if (chunks.length === 0) {
    chunks.push([])
  }
  return chunks.map((chunk, index) => ({
    kind: 'users',
    title: `${t('Users')} (${index + 1}/${chunks.length})`,
    rows: [
      header,
      ...chunk.map((user) => [
        user.department || '—',
        user.display_name || user.username,
        String(user.requests),
        changeText(user.requests, user.prev_requests),
        `${user.active_days} / ${user.prev_active_days}`,
        formatUSD(user.quota),
        changeText(user.quota, user.prev_quota),
      ]),
    ],
  }))
}

// departmentSlides 生成一张部门统计页：总用户、本期/上期活跃用户与活跃率。
function departmentSlides(
  departments: UsageReport['departments'],
  t: TFunction
): ReportSlides[] {
  return [
    {
      kind: 'departments',
      title: t('Departments'),
      rows: [
        [
          t('Department'),
          t('Total users'),
          t('Active users'),
          t('Previous period'),
          t('Active rate'),
        ],
        ...departments.map((department) => [
          department.department,
          String(department.total_users),
          String(department.active_users),
          String(department.prev_active_users),
          `${Math.round(department.active_rate * 100)}%`,
        ]),
      ],
    },
  ]
}

function formatUSD(quota: number): string {
  return `$${quotaUnitsToDollars(quota).toFixed(2)}`
}

export function buildReportSlides(
  report: UsageReport,
  t: TFunction
): ReportSlides[] {
  const periodName =
    report.period === 'week'
      ? t('Usage Weekly Report')
      : t('Usage Monthly Report')

  const overviewRows = [
    [t('Metric'), report.current.label, report.previous.label, t('Change')],
    [
      t('Requests'),
      String(report.overview.requests),
      String(report.overview.prev_requests),
      changeText(report.overview.requests, report.overview.prev_requests),
    ],
    [
      t('Cost'),
      formatUSD(report.overview.quota),
      formatUSD(report.overview.prev_quota),
      changeText(report.overview.quota, report.overview.prev_quota),
    ],
    [
      t('Tokens'),
      String(report.overview.tokens),
      String(report.overview.prev_tokens),
      changeText(report.overview.tokens, report.overview.prev_tokens),
    ],
    [
      t('Active users'),
      String(report.overview.active_users),
      String(report.overview.prev_active_users),
      changeText(
        report.overview.active_users,
        report.overview.prev_active_users
      ),
    ],
  ]

  const topCostUsers = report.users.slice(0, TOP_PIE_USERS)

  return [
    {
      kind: 'title',
      title: `${periodName} ${report.current.label}`,
    },
    { kind: 'overview', title: t('Overview'), rows: overviewRows },
    ...departmentSlides(report.departments, t),
    {
      kind: 'trend',
      title: t('Daily trend'),
      labels: report.daily.map((item) => item.date.slice(5)),
      requestsValues: report.daily.map((item) => item.requests),
      costValues: report.daily.map((item) => quotaUnitsToDollars(item.quota)),
    },
    ...userSlides(report.users, t),
    {
      kind: 'cost-share',
      title: t('Cost distribution'),
      labels: topCostUsers.map((user) => user.display_name || user.username),
      values: topCostUsers.map((user) => quotaUnitsToDollars(user.quota)),
    },
  ]
}

export async function exportUsageReportPptx(
  report: UsageReport,
  t: TFunction
): Promise<string> {
  const pptx = new pptxgenjs()
  pptx.layout = 'LAYOUT_16x9'
  const slides = buildReportSlides(report, t)

  for (const slide of slides) {
    const page = pptx.addSlide()
    page.addText(slide.title, { x: 0.5, y: 0.3, fontSize: 24, bold: true })
    if (slide.kind === 'title') {
      const generatedAt = new Date().toLocaleString(
        toIntlLocale(i18n.resolvedLanguage || i18n.language)
      )
      page.addText(
        `${t('Current period')}: ${report.current.start_date} ~ ${report.current.end_date}\n` +
          `${t('Generated at')}: ${generatedAt}`,
        { x: 0.5, y: 2.0, fontSize: 16 }
      )
    } else if (
      slide.kind === 'overview' ||
      slide.kind === 'departments' ||
      slide.kind === 'users'
    ) {
      page.addTable(
        (slide.rows ?? []).map((row) => row.map((cell) => ({ text: cell }))),
        {
          x: 0.5,
          y: 1.2,
          w: 9.0,
          fontSize: 12,
          border: { pt: 0.5, color: 'CCCCCC' },
        }
      )
    } else if (slide.kind === 'trend') {
      page.addChart(
        pptx.ChartType.bar,
        [
          {
            name: t('Requests'),
            labels: slide.labels ?? [],
            values: slide.requestsValues ?? [],
          },
        ],
        { x: 0.4, y: 1.1, w: 9.0, h: 4.6, showLegend: true }
      )
    } else if (slide.kind === 'cost-share') {
      page.addChart(
        pptx.ChartType.pie,
        [
          {
            name: t('Cost'),
            labels: slide.labels ?? [],
            values: slide.values ?? [],
          },
        ],
        { x: 2.0, y: 1.1, w: 6.0, h: 4.6, showLegend: true, showPercent: true }
      )
    }
  }

  const fileName = `usage-report-${report.current.label}.pptx`
  await pptx.writeFile({ fileName })
  return fileName
}
