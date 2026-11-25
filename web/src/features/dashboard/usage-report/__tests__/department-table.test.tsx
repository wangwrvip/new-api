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
import { render, screen } from '@testing-library/react'
import { describe, expect, it } from 'vitest'

import { DepartmentTable } from '../components/department-table'
import type { UsageReportDepartment } from '../types'

const departments: UsageReportDepartment[] = [
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
]

describe('DepartmentTable', () => {
  it('renders one row per department with totals and rate', () => {
    render(<DepartmentTable departments={departments} />)

    expect(screen.getByRole('cell', { name: '视频部' })).toBeInTheDocument()
    expect(screen.getByRole('cell', { name: '公共部门' })).toBeInTheDocument()
    expect(screen.getByText('10')).toBeInTheDocument() // total users
    expect(screen.getByText('8')).toBeInTheDocument() // active users
    expect(screen.getByText('80%')).toBeInTheDocument() // rate
    expect(screen.getByText('0%')).toBeInTheDocument()
  })

  it('compares active users with the previous period', () => {
    render(<DepartmentTable departments={departments} />)

    // 视频部：8 vs 4 → +100%；公共部门：0 vs 2 → -100%
    expect(screen.getByText(/↑ \+100\.0%/)).toBeInTheDocument()
    expect(screen.getByText(/↓ -100\.0%/)).toBeInTheDocument()
  })

  it('renders empty state when no departments', () => {
    render(<DepartmentTable departments={[]} />)

    expect(
      screen.getByText('No usage records in this period')
    ).toBeInTheDocument()
  })
})
