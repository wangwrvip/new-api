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
import type { TFunction } from 'i18next'
import { describe, expect, it } from 'vitest'

import { buildReportSlides } from '../lib/export-ppt'
import type { UsageReport } from '../types'

const identityT = ((key: string) => key) as unknown as TFunction

function buildReport(userCount: number): UsageReport {
  const all = Array.from({ length: 60 }, (_, i) => ({
    user_id: i + 1,
    username: `user-${String(i + 1).padStart(2, '0')}`,
    department: '视频部',
    requests: 100 - i,
    prev_requests: 50,
    active_days: 5,
    prev_active_days: 4,
    quota: 100000 * (20 - i),
    prev_quota: 50000,
    tokens: 1000,
    prev_tokens: 500,
  }))

  return {
    period: 'week',
    current: {
      label: '2026-W36',
      start_date: '2026-08-31',
      end_date: '2026-09-06',
    },
    previous: {
      label: '2026-W35',
      start_date: '2026-08-24',
      end_date: '2026-08-30',
    },
    overview: {
      requests: 300,
      prev_requests: 200,
      quota: 600000,
      prev_quota: 500000,
      tokens: 9000,
      prev_tokens: 0,
      active_users: 8,
      prev_active_users: 4,
    },
    daily: Array.from({ length: 7 }, (_, i) => ({
      date: `2026-09-0${i + 1}`,
      requests: 10 * (i + 1),
      quota: 1000,
      active_users: 2,
    })),
    departments: [
      {
        department: '视频部',
        total_users: 10,
        active_users: 8,
        prev_active_users: 4,
        active_rate: 0.8,
      },
      {
        department: '公共部门',
        total_users: 5,
        active_users: 0,
        prev_active_users: 2,
        active_rate: 0,
      },
    ],
    users: all.slice(0, userCount),
  }
}

describe('buildReportSlides', () => {
  it('shows all users in a single table when they fit on one slide', () => {
    const slides = buildReportSlides(buildReport(10), identityT)
    const userSlides = slides.filter((slide) => slide.kind === 'users')

    expect(userSlides).toHaveLength(1) // 10 ≤ 每页上限 → 一页
    expect(userSlides[0].title).toBe('Users (1/1)')
    const rows = userSlides[0].rows ?? []
    expect(rows).toHaveLength(11) // 表头 + 10 用户，全部展示、无汇总行
    expect(rows.some((row) => row[0].startsWith('Others'))).toBe(false)
    expect(rows[0][0]).toBe('Department') // 首列为部门
  })

  it('paginates users across multiple slides with page numbers', () => {
    const slides = buildReportSlides(buildReport(41), identityT)
    const userSlides = slides.filter((slide) => slide.kind === 'users')

    expect(userSlides).toHaveLength(5) // 每页 10 人：10/10/10/10/1
    expect(userSlides.map((slide) => slide.title)).toEqual([
      'Users (1/5)',
      'Users (2/5)',
      'Users (3/5)',
      'Users (4/5)',
      'Users (5/5)',
    ])
    expect(userSlides[0].rows).toHaveLength(11) // 表头 + 10
    expect(userSlides[1].rows).toHaveLength(11)
    expect(userSlides[2].rows).toHaveLength(11)
    expect(userSlides[3].rows).toHaveLength(11)
    expect(userSlides[4].rows).toHaveLength(2) // 表头 + 1
    const totalUsers = userSlides.reduce(
      (acc, slide) => acc + (slide.rows?.length ?? 1) - 1,
      0
    )
    expect(totalUsers).toBe(41) // 分页不丢用户
  })

  it('prefers display name over username in table and cost share', () => {
    const report = buildReport(1)
    report.users[0].display_name = 'Alice A'
    const slides = buildReportSlides(report, identityT)
    const rows = slides.find((slide) => slide.kind === 'users')?.rows ?? []

    expect(rows[1][1]).toBe('Alice A') // 第 1 列是部门，第 2 列是用户
    expect(rows[1][0]).toBe('视频部')
    const costSlide = slides.find((slide) => slide.kind === 'cost-share')
    expect(costSlide?.labels?.[0]).toBe('Alice A')
  })

  it('builds trend chart from daily entries', () => {
    const slides = buildReportSlides(buildReport(3), identityT)
    const trendSlide = slides.find((slide) => slide.kind === 'trend')

    expect(trendSlide?.labels).toHaveLength(7)
    expect(trendSlide?.labels?.[0]).toBe('09-01')
    expect(trendSlide?.requestsValues?.[0]).toBe(10)
  })

  it('caps cost share pie at top 10 users', () => {
    const slides = buildReportSlides(buildReport(20), identityT)
    const costSlide = slides.find((slide) => slide.kind === 'cost-share')

    expect(costSlide?.labels).toHaveLength(10)
    expect(costSlide?.labels?.[0]).toBe('user-01')
  })

  it('builds a department stats slide with totals and rate', () => {
    const slides = buildReportSlides(buildReport(3), identityT)
    const deptSlide = slides.find((slide) => slide.kind === 'departments')

    expect(deptSlide?.rows?.[0]).toEqual([
      'Department',
      'Total users',
      'Active users',
      'Previous period',
      'Active rate',
    ])
    expect(deptSlide?.rows?.[1]).toEqual(['视频部', '10', '8', '4', '80%'])
  })

  it('renders six slides starting with the title page', () => {
    const slides = buildReportSlides(buildReport(3), identityT)

    expect(slides.map((slide) => slide.kind)).toEqual([
      'title',
      'overview',
      'departments',
      'trend',
      'users',
      'cost-share',
    ])
    expect(slides[0].title).toContain('Usage Weekly Report')
    expect(slides[0].title).toContain('2026-W36')
  })
})
