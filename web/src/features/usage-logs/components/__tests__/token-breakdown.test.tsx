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
import {
  flexRender,
  getCoreRowModel,
  useReactTable,
} from '@tanstack/react-table'
import { render, screen } from '@testing-library/react'
import { afterEach, describe, expect, test } from 'vitest'

import type { UsageLog } from '../../data/schema'
import type { LogOtherData } from '../../types'
import { useCommonLogsColumns } from '../columns/common-logs-columns'
import { DetailsDialog } from '../dialogs/details-dialog'

const queryClients: QueryClient[] = []

function makeLog(fields: {
  other: LogOtherData
  prompt_tokens?: number
  completion_tokens?: number
  input_tokens?: number
  cache_read_tokens?: number
}): UsageLog {
  return {
    id: 1,
    user_id: 1,
    created_at: 1,
    type: 2,
    content: '',
    username: 'user',
    token_name: 'token',
    model_name: 'gpt-test',
    quota: 0,
    prompt_tokens: fields.prompt_tokens ?? 0,
    completion_tokens: fields.completion_tokens ?? 0,
    input_tokens: fields.input_tokens ?? 0,
    cache_tokens: 0,
    cache_read_tokens: fields.cache_read_tokens ?? 0,
    use_time: 0,
    is_stream: false,
    channel: 1,
    channel_name: '',
    token_id: 1,
    group: 'default',
    ip: '',
    other: JSON.stringify(fields.other),
    request_id: 'req-1',
    upstream_request_id: '',
  }
}

function renderDetails(log: UsageLog): void {
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
  queryClients.push(queryClient)
  render(
    <QueryClientProvider client={queryClient}>
      <DetailsDialog
        log={log}
        isAdmin={false}
        isRoot={false}
        open
        onOpenChange={() => undefined}
      />
    </QueryClientProvider>
  )
}

function rowValue(label: string): string | null {
  return screen.getByText(label).nextElementSibling?.textContent ?? null
}

function TokensCell(props: { log: UsageLog }) {
  const table = useReactTable({
    data: [props.log],
    columns: useCommonLogsColumns(false, false),
    getCoreRowModel: getCoreRowModel(),
  })
  const cell = table
    .getRowModel()
    .rows[0].getAllCells()
    .find((item) => item.column.id === 'prompt_tokens')
  if (!cell) throw new Error('The log list must have a tokens column')
  return <>{flexRender(cell.column.columnDef.cell, cell.getContext())}</>
}

afterEach(() => {
  for (const queryClient of queryClients) queryClient.clear()
  queryClients.length = 0
})

describe('token breakdown', () => {
  test('shows OpenAI input without cache hits and the cache read separately', () => {
    renderDetails(
      makeLog({
        prompt_tokens: 494934,
        completion_tokens: 1,
        input_tokens: 197206,
        cache_read_tokens: 297728,
        other: {
          cache_tokens: 297728,
          estimated_prompt_tokens: 494938,
        },
      })
    )

    expect(rowValue('Input Tokens')).toBe('197,206')
    expect(rowValue('Cache Read')).toBe('297,728')
    expect(rowValue('Estimated / Actual')).toMatch(/^494,938 \/ 494,934 \(/)
  })

  test('shows the same Anthropic breakdown and counts cache hits in the actual context', () => {
    renderDetails(
      makeLog({
        prompt_tokens: 197206,
        completion_tokens: 1,
        input_tokens: 197206,
        cache_read_tokens: 297728,
        other: {
          cache_tokens: 297728,
          estimated_prompt_tokens: 494933,
          claude: true,
        },
      })
    )

    expect(rowValue('Input Tokens')).toBe('197,206')
    expect(rowValue('Cache Read')).toBe('297,728')
    expect(rowValue('Estimated / Actual')).toMatch(/^494,933 \/ 494,934 \(/)
  })

  test('derives the input of legacy OpenAI logs by subtracting cache hits', () => {
    renderDetails(
      makeLog({
        prompt_tokens: 494934,
        other: {
          cache_tokens: 297728,
          estimated_prompt_tokens: 494938,
        },
      })
    )

    expect(rowValue('Input Tokens')).toBe('197,206')
    expect(rowValue('Cache Read')).toBe('297,728')
    expect(rowValue('Estimated / Actual')).toMatch(/^494,938 \/ 494,934 \(/)
  })

  test('keeps the prompt count of legacy Anthropic logs because it already excludes cache hits', () => {
    renderDetails(
      makeLog({
        prompt_tokens: 197206,
        other: {
          cache_tokens: 297728,
          estimated_prompt_tokens: 494933,
          claude: true,
        },
      })
    )

    expect(rowValue('Input Tokens')).toBe('197,206')
    expect(rowValue('Cache Read')).toBe('297,728')
    expect(rowValue('Estimated / Actual')).toMatch(/^494,933 \/ 494,934 \(/)
  })

  test('renders the list column with input excluding cache and a separate cache badge', () => {
    render(
      <TokensCell
        log={makeLog({
          prompt_tokens: 494934,
          completion_tokens: 1,
          input_tokens: 197206,
          cache_read_tokens: 297728,
          other: { cache_tokens: 297728 },
        })}
      />
    )

    expect(screen.getByText('197,206 / 1')).toBeVisible()
    expect(screen.getByText('Cache↓ 297,728')).toBeVisible()
    expect(screen.queryByText('494,934 / 1')).toBeNull()
  })
})
