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
import {
  flexRender,
  getCoreRowModel,
  useReactTable,
} from '@tanstack/react-table'
import { render, screen } from '@testing-library/react'
import i18next from 'i18next'
import { afterEach, beforeAll, describe, expect, test } from 'vitest'

import type { UsageLog } from '../../data/schema'
import type { LogOtherData } from '../../types'
import {
  buildDetailSegments,
  useCommonLogsColumns,
} from '../columns/common-logs-columns'
import { DetailsDialog } from '../dialogs/details-dialog'

const i18nKeys = {
  'Log Details': 'Log Details',
  'Request Preview': 'Request Preview',
  'Response Preview': 'Response Preview',
  'Last User Message': 'Last User Message',
  'Estimated / Actual': 'Estimated / Actual',
  'Token Breakdown': 'Token Breakdown',
  'Context limit exceeded': 'Context limit exceeded',
  'Click to view full details': 'Click to view full details',
  tokens: 'tokens',
  Minute: 'Minute',
  '{{start}}–{{end}}': '{{start}}–{{end}}',
  '{{condition}} ({{timezone}})': '{{condition}} ({{timezone}})',
}

function makeLog(other: LogOtherData, type = 2): UsageLog {
  return {
    id: 1,
    user_id: 1,
    created_at: 1,
    type,
    content: '',
    username: 'user',
    token_name: 'token',
    model_name: 'gpt-4o',
    quota: 5000,
    prompt_tokens: 10,
    completion_tokens: 5,
    input_tokens: 0,
    cache_tokens: 0,
    cache_read_tokens: 0,
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

function DetailColumnPreview(props: { other: LogOtherData; type?: number }) {
  const table = useReactTable({
    data: [{ ...makeLog(props.other), type: props.type ?? 2 }],
    columns: useCommonLogsColumns(false, false),
    getCoreRowModel: getCoreRowModel(),
  })
  const cell = table
    .getRowModel()
    .rows[0].getAllCells()
    .find((item) => item.column.id === 'content')
  if (!cell) throw new Error('The log must have a content column')
  return flexRender(cell.column.columnDef.cell, cell.getContext())
}

function renderDetailColumn(other: LogOtherData, type?: number): QueryClient {
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
      <DetailColumnPreview other={other} type={type} />
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

  test('renders the last user message preview for the log owner', () => {
    queryClients.push(
      renderDetails(
        {
          group_ratio: 1,
          request_preview_user: '最后一条用户消息',
        },
        false
      )
    )

    expect(screen.getByText('Last User Message')).toBeInTheDocument()
    expect(screen.getByText('最后一条用户消息')).toBeInTheDocument()
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

  test('ignores nested admin_info previews and admin fields in non-admin views', () => {
    queryClients.push(
      renderDetails(
        {
          group_ratio: 1,
          request_preview: '请求文本预览',
          request_preview_user: '最后一条用户消息',
          response_preview: '模型输出的第一段内容',
          admin_info: {
            request_preview: '管理员专用请求预览',
            request_preview_user: '管理员专用用户消息',
            response_preview: '管理员专用输出预览',
            reject_reason: 'blocked by channel policy',
            quota_saturation: {
              op: 'round',
              kind: 'overflow',
              original: 3e9,
              clamped: 2147483647,
            },
          },
        },
        false
      )
    )

    expect(screen.getByText('请求文本预览')).toBeInTheDocument()
    expect(screen.getByText('最后一条用户消息')).toBeInTheDocument()
    expect(screen.getByText('模型输出的第一段内容')).toBeInTheDocument()
    expect(screen.queryByText('管理员专用请求预览')).toBeNull()
    expect(screen.queryByText('管理员专用用户消息')).toBeNull()
    expect(screen.queryByText('管理员专用输出预览')).toBeNull()
    expect(screen.queryByText('Reject Reason')).toBeNull()
    expect(screen.queryByText('blocked by channel policy')).toBeNull()
    expect(screen.queryByText('Quota clamped')).toBeNull()
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

  test('marks context limit errors in danger styling with an extra detail', () => {
    queryClients.push(
      renderDetailColumn(
        {
          error_code: 'context_limit_exceeded',
          estimated_prompt_tokens: 120,
          context_limit: 32000,
        },
        5
      )
    )

    expect(screen.getByText('Context limit exceeded')).toHaveClass(
      'text-red-600'
    )
    expect(screen.getByText('+1')).toBeInTheDocument()
  })

  test('uses the translated token unit in context limit error segments', () => {
    const segments = buildDetailSegments(
      makeLog({ error_code: 'context_limit_exceeded' }, 5),
      {
        error_code: 'context_limit_exceeded',
        estimated_prompt_tokens: 120,
        context_limit: 32000,
      },
      (key) => (key === 'tokens' ? 'Token' : key),
      false,
      'en'
    )

    expect(segments[0]).toEqual({
      text: 'Context limit exceeded',
      danger: true,
    })
    expect(segments[1]?.text).toBe('120 / 32000 Token')
  })

  test('formats minute request rules as minute ranges with a timezone', () => {
    const segments = buildDetailSegments(
      makeLog({ billing_mode: 'tiered_expr' }),
      {
        billing_mode: 'tiered_expr',
        matched_tier: 'peak',
        expr_b64: btoa('tier("peak", 10000)'),
        request_rules: [
          {
            cond: 'minute("Asia/Shanghai") >= 30 && minute("Asia/Shanghai") < 45',
            multiplier: 1.5,
            matched: true,
          },
        ],
      },
      i18next.t.bind(i18next),
      false,
      'en'
    )

    expect(segments.map((segment) => segment.text)).toContain(
      'Minute: 30–44 (Asia/Shanghai) ×1.5'
    )
    expect(segments.some((segment) => segment.text.includes('30:00'))).toBe(
      false
    )
  })
})
