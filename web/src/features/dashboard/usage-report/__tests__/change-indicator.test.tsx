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
import { render } from '@testing-library/react'
import { describe, expect, it } from 'vitest'

import { ChangeIndicator } from '../components/change-indicator'

describe('ChangeIndicator', () => {
  it.each([
    { current: 120, previous: 100, expected: '↑ +20.0%' },
    { current: 100, previous: 120, expected: '↓ -16.7%' },
    { current: 100, previous: 100, expected: '0.0%' },
  ])(
    'renders $current against $previous as "$expected"',
    ({ current, previous, expected }) => {
      const { container } = render(
        <ChangeIndicator current={current} previous={previous} />
      )

      expect(container.textContent).toBe(expected)
    }
  )
})
