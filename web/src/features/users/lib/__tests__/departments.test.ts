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

import {
  DEFAULT_DEPARTMENT,
  FALLBACK_DEPARTMENTS,
  parseDepartments,
} from '../departments'

describe('parseDepartments', () => {
  it('falls back to the default list when the raw value is undefined', () => {
    expect(parseDepartments(undefined)).toEqual(FALLBACK_DEPARTMENTS)
  })

  it('parses a valid JSON string array', () => {
    expect(parseDepartments('["视频部","测试组"]')).toEqual([
      '视频部',
      '测试组',
    ])
  })

  it('falls back to the default list on malformed JSON', () => {
    expect(parseDepartments('not-json')).toEqual(FALLBACK_DEPARTMENTS)
  })

  it('falls back to the default list on non-array JSON', () => {
    expect(parseDepartments('{"a":1}')).toEqual(FALLBACK_DEPARTMENTS)
  })

  it('falls back to the default list when the array is empty', () => {
    expect(parseDepartments('[]')).toEqual(FALLBACK_DEPARTMENTS)
  })

  it('trims names and drops blank entries', () => {
    expect(parseDepartments('[" 视频部 ", " ", "测试组"]')).toEqual([
      '视频部',
      '测试组',
    ])
  })

  it('returns a copy so callers cannot mutate the fallback list', () => {
    const parsed = parseDepartments(undefined)
    expect(parsed).not.toBe(FALLBACK_DEPARTMENTS)
    parsed.pop()
    expect(FALLBACK_DEPARTMENTS).toContain(DEFAULT_DEPARTMENT)
  })

  it('keeps the default department first in the fallback list', () => {
    expect(FALLBACK_DEPARTMENTS[0]).toBe(DEFAULT_DEPARTMENT)
  })
})
