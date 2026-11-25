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

import { formatPercentChange, percentChange } from '../lib/stats'

interface ChangeIndicatorProps {
  current: number
  previous: number
}

export function ChangeIndicator(props: ChangeIndicatorProps) {
  const { t } = useTranslation()
  const formatted = formatPercentChange(
    percentChange(props.current, props.previous)
  )
  if (formatted == null) {
    return (
      <span title={t('No usage in previous period')} aria-label={t('Change')}>
        —
      </span>
    )
  }
  const direction = formatted.startsWith('+') ? '↑' : '↓'
  return (
    <span aria-label={`${t('Change')} ${formatted}`}>
      {direction} {formatted}
    </span>
  )
}
