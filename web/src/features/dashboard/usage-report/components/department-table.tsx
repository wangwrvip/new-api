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
import { useTranslation } from 'react-i18next'

import { toIntlLocale } from '@/i18n/languages'
import { formatNumber } from '@/lib/format'

import type { UsageReportDepartment } from '../types'
import { ChangeIndicator } from './change-indicator'

interface DepartmentTableProps {
  departments: UsageReportDepartment[]
}

/** active_rate 为 0..1 的小数，展示为整数百分比。 */
function formatRate(rate: number): string {
  return `${Math.round(rate * 100)}%`
}

export function DepartmentTable(props: DepartmentTableProps) {
  const { t, i18n } = useTranslation()
  const locale = toIntlLocale(i18n.resolvedLanguage || i18n.language)

  if (props.departments.length === 0) {
    return (
      <div className='text-muted-foreground py-8 text-center' role='status'>
        {t('No usage records in this period')}
      </div>
    )
  }

  return (
    <div className='overflow-x-auto'>
      <table className='w-full text-sm'>
        <thead>
          <tr className='text-muted-foreground border-b text-left'>
            <th className='p-2'>{t('Department')}</th>
            <th className='p-2'>{t('Total users')}</th>
            <th className='p-2'>{t('Active users')}</th>
            <th className='p-2'>{t('Change')}</th>
            <th className='p-2'>{t('Active rate')}</th>
          </tr>
        </thead>
        <tbody>
          {props.departments.map((department) => (
            <tr key={department.department} className='border-b'>
              <td className='p-2 font-medium'>{department.department}</td>
              <td className='p-2'>
                {formatNumber(department.total_users, locale)}
              </td>
              <td className='p-2'>
                {formatNumber(department.active_users, locale)}
              </td>
              <td className='p-2'>
                <ChangeIndicator
                  current={department.active_users}
                  previous={department.prev_active_users}
                />
                <span className='text-muted-foreground'>
                  {' '}
                  / {formatNumber(department.prev_active_users, locale)}
                </span>
              </td>
              <td className='p-2'>{formatRate(department.active_rate)}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  )
}
