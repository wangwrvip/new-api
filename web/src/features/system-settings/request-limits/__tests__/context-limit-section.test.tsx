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
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import i18next from 'i18next'
import { beforeAll, beforeEach, describe, expect, test, vi } from 'vitest'

import { updateSystemOption } from '../../api'
import { SettingsPageProvider } from '../../components/settings-page-context'
import { ContextLimitSection } from '../context-limit-section'
import {
  formatModelLimitLines,
  isValidModelLimitText,
  parseModelLimitLines,
} from '../context-limit-utils'

vi.mock('../../api', async (importOriginal) => {
  const actual = await importOriginal<typeof import('../../api')>()
  return { ...actual, updateSystemOption: vi.fn() }
})

const updateOptionMock = vi.mocked(updateSystemOption)

type SectionDefaults = {
  'context_limit_setting.enabled': boolean
  'context_limit_setting.default_max_prompt_tokens': number
  'context_limit_setting.model_max_prompt_tokens': string
}

const baseDefaults: SectionDefaults = {
  'context_limit_setting.enabled': false,
  'context_limit_setting.default_max_prompt_tokens': 0,
  'context_limit_setting.model_max_prompt_tokens': '{}',
}

function renderSection(defaultValues: SectionDefaults) {
  const queryClient = new QueryClient({
    defaultOptions: { mutations: { retry: false } },
  })
  const actionsContainer = document.createElement('div')
  document.body.appendChild(actionsContainer)

  const utils = render(
    <QueryClientProvider client={queryClient}>
      <SettingsPageProvider
        actionsContainer={actionsContainer}
        titleStatusContainer={null}
      >
        <ContextLimitSection defaultValues={defaultValues} />
      </SettingsPageProvider>
    </QueryClientProvider>
  )

  return { ...utils, queryClient, actionsContainer }
}

describe('model limit line helpers', () => {
  test('parses model=limit lines into a map ignoring blank lines', () => {
    expect(
      parseModelLimitLines('glm-4.7=210000\n\nclaude-sonnet-5 = 128000\n')
    ).toEqual({
      'glm-4.7': 210000,
      'claude-sonnet-5': 128000,
    })
  })

  test('formats a map into sorted stable lines', () => {
    expect(
      formatModelLimitLines({
        'glm-4.7': 210000,
        'claude-sonnet-5': 128000,
      })
    ).toBe('claude-sonnet-5=128000\nglm-4.7=210000')
  })

  test('validation accepts blank lines and model=integer pairs only', () => {
    expect(isValidModelLimitText('')).toBe(true)
    expect(isValidModelLimitText('glm-4.7=210000\n   \nglm-4.6=0')).toBe(true)
    expect(isValidModelLimitText('glm-4.7 210000')).toBe(false)
    expect(isValidModelLimitText('glm-4.7=-1')).toBe(false)
    expect(isValidModelLimitText('glm 4.7=210000')).toBe(false)
  })
})

describe('context limit section', () => {
  beforeAll(() => {
    i18next.addResourceBundle('en', 'translation', {
      'Context Limits': 'Context Limits',
      'Enable context limits': 'Enable context limits',
      'Default max prompt tokens': 'Default max prompt tokens',
      'Per-model max prompt tokens': 'Per-model max prompt tokens',
      'Save context limits': 'Save context limits',
    })
  })

  beforeEach(() => {
    updateOptionMock.mockReset()
    updateOptionMock.mockResolvedValue({ success: true } as never)
  })

  test('renders per-model limits as lines from the stored JSON map', () => {
    renderSection({
      'context_limit_setting.enabled': false,
      'context_limit_setting.default_max_prompt_tokens': 128000,
      'context_limit_setting.model_max_prompt_tokens': '{"glm-4.7":210000}',
    })

    expect(
      screen.getByRole('switch', { name: 'Enable context limits' })
    ).not.toBeChecked()
    expect(
      screen.getByRole('spinbutton', { name: 'Default max prompt tokens' })
    ).toHaveValue(128000)
    expect(
      screen.getByRole('textbox', { name: 'Per-model max prompt tokens' })
    ).toHaveValue('glm-4.7=210000')
  })

  test('marks the form invalid and disables saving when a line is malformed', async () => {
    renderSection(baseDefaults)

    const textarea = screen.getByRole('textbox', {
      name: 'Per-model max prompt tokens',
    })
    fireEvent.change(textarea, { target: { value: 'glm-4.7 not-a-number' } })

    await waitFor(() =>
      expect(textarea).toHaveAttribute('aria-invalid', 'true')
    )
    expect(
      screen.getByRole('button', { name: 'Save context limits' })
    ).toBeDisabled()
  })

  test('saves only changed keys with the map serialized as canonical JSON', async () => {
    renderSection(baseDefaults)

    fireEvent.click(
      screen.getByRole('switch', { name: 'Enable context limits' })
    )
    fireEvent.change(
      screen.getByRole('spinbutton', { name: 'Default max prompt tokens' }),
      { target: { value: '210000' } }
    )
    fireEvent.change(
      screen.getByRole('textbox', { name: 'Per-model max prompt tokens' }),
      { target: { value: 'glm-4.7=210000' } }
    )

    const saveButton = screen.getByRole('button', {
      name: 'Save context limits',
    })
    await waitFor(() => expect(saveButton).toBeEnabled())
    fireEvent.click(saveButton)

    await waitFor(() => expect(updateOptionMock).toHaveBeenCalledTimes(3))
    expect(updateOptionMock).toHaveBeenCalledWith({
      key: 'context_limit_setting.enabled',
      value: true,
    })
    expect(updateOptionMock).toHaveBeenCalledWith({
      key: 'context_limit_setting.default_max_prompt_tokens',
      value: 210000,
    })
    expect(updateOptionMock).toHaveBeenCalledWith({
      key: 'context_limit_setting.model_max_prompt_tokens',
      value: '{"glm-4.7":210000}',
    })
  })
})
