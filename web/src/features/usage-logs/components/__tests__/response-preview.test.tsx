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
along with this program.  If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { render, screen } from '@testing-library/react'
import i18next from 'i18next'
import { afterEach, beforeAll, describe, expect, test } from 'vitest'

import type { UsageLog } from '../../data/schema'
import type { LogOtherData } from '../../types'
import { DetailsDialog } from '../dialogs/details-dialog'

const i18nKeys = {
  'Log Details': 'Log Details',
  'Request Preview': 'Request Preview',
  'Response Preview': 'Response Preview',
  'Last User Message': 'Last User Message',
  'Estimated / Actual': 'Estimated / Actual',
  'Token Breakdown': 'Token Breakdown',
}

function makeLog(other: LogOtherData): UsageLog {
  return {
    id: 1,
    user_id: 1,
    created_at: 1,
    type: 2,
    content: '',
    username: 'user',
    token_name: 'token',
    model_name: 'gpt-4o',
    quota: 5000,
    prompt_tokens: 10,
    completion_tokens: 5,
    use_time: 0,
    is_stream: false,
    channel: 1,
    channel_name: '',
    token_id: 1,
    group: 'default',
    ip: '',
    other: JSON.stringify(other),
    request_id: 'req-1',
    upstream_request_id: '',
  }
}

function renderDetails(
  other: LogOtherData,
  isAdmin: boolean,
  logOverrides: Partial<UsageLog> = {}
): QueryClient {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })
  const freshAt = Date.now() + 60_000
  queryClient.setQueryData(['status'], {}, { updatedAt: freshAt })
  queryClient.setQueryData(
    ['pricing'],
    { data: [], vendors: [] },
    { updatedAt: freshAt }
  )

  render(
    <QueryClientProvider client={queryClient}>
      <DetailsDialog
        log={{ ...makeLog(other), ...logOverrides }}
        isAdmin={isAdmin}
        isRoot={false}
        open
        onOpenChange={() => undefined}
      />
    </QueryClientProvider>
  )
  return queryClient
}

describe('details dialog response preview', () => {
  const queryClients: QueryClient[] = []

  beforeAll(() => {
    i18next.addResourceBundle('en', 'translation', i18nKeys)
  })

  afterEach(() => {
    for (const queryClient of queryClients) {
      queryClient.clear()
    }
    queryClients.length = 0
  })

  test('renders top-level previews for the log owner (non-admin)', () => {
    queryClients.push(
      renderDetails(
        {
          group_ratio: 1,
          request_preview: '请求文本预览',
          response_preview: '模型输出的第一段内容',
        },
        false
      )
    )

    expect(screen.getByText('Request Preview')).toBeInTheDocument()
    expect(screen.getByText('请求文本预览')).toBeInTheDocument()
    expect(screen.getByText('Response Preview')).toBeInTheDocument()
    expect(screen.getByText('模型输出的第一段内容')).toBeInTheDocument()
  })

  test('falls back to legacy admin_info previews for admins viewing old logs', () => {
    queryClients.push(
      renderDetails(
        {
          group_ratio: 1,
          admin_info: {
            request_preview: '旧的请求预览',
            response_preview: '旧的输出预览',
          },
        },
        true
      )
    )

    expect(screen.getByText('旧的请求预览')).toBeInTheDocument()
    expect(screen.getByText('旧的输出预览')).toBeInTheDocument()
  })

  test('renders no preview blocks when the fields are absent', () => {
    queryClients.push(
      renderDetails(
        {
          group_ratio: 1,
        },
        false
      )
    )

    expect(screen.queryByText('Request Preview')).toBeNull()
    expect(screen.queryByText('Response Preview')).toBeNull()
  })

  test('adds cache reads to the actual prompt count for Claude requests', () => {
    queryClients.push(
      renderDetails(
        {
          group_ratio: 1,
          claude: true,
          cache_tokens: 300,
          estimated_prompt_tokens: 1200,
        },
        false,
        { prompt_tokens: 1000 }
      )
    )

    expect(screen.getByText('Estimated / Actual')).toBeInTheDocument()
    expect(screen.getByText('1,200 / 1,300 (-7.69%)')).toBeInTheDocument()
  })

  test('uses the recorded prompt count directly for non-Claude requests', () => {
    queryClients.push(
      renderDetails(
        {
          group_ratio: 1,
          cache_tokens: 300,
          estimated_prompt_tokens: 1200,
        },
        false,
        { prompt_tokens: 1000 }
      )
    )

    expect(screen.getByText('1,200 / 1,000 (+20%)')).toBeInTheDocument()
  })
})
