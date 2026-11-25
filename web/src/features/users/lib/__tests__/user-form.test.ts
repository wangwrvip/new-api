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

import type { User } from '../../types'
import { DEFAULT_DEPARTMENT } from '../departments'
import {
  transformFormDataToPayload,
  transformUserToFormDefaults,
  USER_FORM_DEFAULT_VALUES,
} from '../user-form'

const baseUser: User = {
  id: 7,
  username: 'alice',
  display_name: 'Alice',
  quota: 0,
  used_quota: 0,
  request_count: 0,
  group: 'default',
  department: DEFAULT_DEPARTMENT,
  status: 1,
  role: 1,
}

describe('transformFormDataToPayload department', () => {
  it('sends the selected department when creating a user', () => {
    const payload = transformFormDataToPayload({
      ...USER_FORM_DEFAULT_VALUES,
      username: 'alice',
      department: '视频部',
    })

    expect(payload.department).toBe('视频部')
  })

  it('sends the selected department when updating a user', () => {
    const payload = transformFormDataToPayload(
      {
        ...USER_FORM_DEFAULT_VALUES,
        username: 'alice',
        department: '视频部',
      },
      7
    )

    expect(payload.id).toBe(7)
    expect(payload.department).toBe('视频部')
  })

  it('falls back to the default department when the form value is empty', () => {
    const payload = transformFormDataToPayload({
      ...USER_FORM_DEFAULT_VALUES,
      username: 'alice',
      department: '',
    })

    expect(payload.department).toBe(DEFAULT_DEPARTMENT)
  })
})

describe('transformUserToFormDefaults department', () => {
  it('reads the department from the user being edited', () => {
    const defaults = transformUserToFormDefaults({
      ...baseUser,
      department: '视频部',
    })

    expect(defaults.department).toBe('视频部')
  })

  it('falls back to the default department when the user has none', () => {
    const defaults = transformUserToFormDefaults({
      ...baseUser,
      department: undefined,
    })

    expect(defaults.department).toBe(DEFAULT_DEPARTMENT)
  })
})
