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

import { cacheHitRate } from '@/features/dashboard/lib'
import { toIntlLocale } from '@/i18n/languages'
import { formatNumber, formatQuota } from '@/lib/format'

import type { UsageReportUser } from '../types'
import { ChangeIndicator } from './change-indicator'

interface UserTableProps {
  users: UsageReportUser[]
}

export function UserTable(props: UserTableProps) {
  const { t, i18n } = useTranslation()
  const locale = toIntlLocale(i18n.resolvedLanguage || i18n.language)

  if (props.users.length === 0) {
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
            <th className='p-2'>{t('User')}</th>
            <th className='p-2'>{t('Requests')}</th>
            <th className='p-2'>{t('Change')}</th>
            <th className='p-2'>{t('Active days')}</th>
            <th className='p-2'>{t('Cost')}</th>
            <th className='p-2'>{t('Tokens')}</th>
          </tr>
        </thead>
        <tbody>
          {props.users.map((user) => (
            <tr key={user.user_id} className='border-b'>
              <td className='p-2'>{user.department || '—'}</td>
              <td className='p-2 font-medium'>
                {user.display_name || user.username}
              </td>
              <td className='p-2'>{formatNumber(user.requests, locale)}</td>
              <td className='p-2'>
                <ChangeIndicator
                  current={user.requests}
                  previous={user.prev_requests}
                />
              </td>
              <td className='p-2'>
                {user.active_days}
                <span className='text-muted-foreground'>
                  {' '}
                  / {user.prev_active_days}
                </span>
              </td>
              <td className='p-2'>
                {formatQuota(user.quota)}
                <span className='text-muted-foreground'>
                  {' '}
                  / {formatQuota(user.prev_quota)}
                </span>
              </td>
              <td className='p-2'>
                {formatNumber(user.tokens, locale)}
                {(user.input_tokens || 0) +
                  (user.output_tokens || 0) +
                  (user.cache_tokens || 0) >
                  0 && (
                  <div className='text-muted-foreground/60 text-xs'>
                    {t('Input')} {formatNumber(user.input_tokens || 0, locale)}{' '}
                    · {t('Output')}{' '}
                    {formatNumber(user.output_tokens || 0, locale)} ·{' '}
                    {t('Cache')} {formatNumber(user.cache_tokens || 0, locale)}
                    {(user.cache_read_tokens || 0) > 0 &&
                      ` · ${(
                        cacheHitRate(
                          user.cache_read_tokens || 0,
                          user.input_tokens || 0,
                          user.cache_tokens || 0
                        ) * 100
                      ).toFixed(1)}%`}
                  </div>
                )}
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  )
}
