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
import { describe, expect, it } from 'vitest'

import { formatPercentChange, percentChange } from '../lib/stats'

describe('percentChange', () => {
  it('returns percentage growth relative to previous', () => {
    expect(percentChange(150, 100)).toBe(50)
  })

  it('returns negative percentage for decline', () => {
    expect(percentChange(80, 100)).toBe(-20)
  })

  it('returns null when previous is zero (no baseline)', () => {
    expect(percentChange(10, 0)).toBeNull()
  })

  it('returns null when previous is negative', () => {
    expect(percentChange(10, -1)).toBeNull()
  })
})

describe('formatPercentChange', () => {
  it('formats with explicit plus sign and one decimal', () => {
    expect(formatPercentChange(12.34)).toBe('+12.3%')
  })

  it('keeps negative sign', () => {
    expect(formatPercentChange(-8)).toBe('-8.0%')
  })

  it('returns null for null input, caller renders placeholder', () => {
    expect(formatPercentChange(null)).toBeNull()
  })
})
