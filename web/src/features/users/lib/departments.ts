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
import { api } from '@/lib/api'

export const DEFAULT_DEPARTMENT = '公共部门'

// 与后端 setting/department_setting.go 的默认清单保持一致；设置项缺失或非法时回退。
export const FALLBACK_DEPARTMENTS = [
  DEFAULT_DEPARTMENT,
  '视频部',
  '单片机',
  '路由网关',
  '测试组',
  '硬件部',
  '工控机部',
  '平台部',
]

const DEPARTMENT_LIST_KEY = 'DepartmentList'

type SystemOption = { key: string; value: string }

/**
 * 解析部门设置项（JSON 字符串数组）。缺失、非法 JSON 或空数组时回退到默认清单，
 * 保证部门下拉与管理部门弹窗始终有可用数据。
 */
export function parseDepartments(raw: string | undefined): string[] {
  if (!raw) return [...FALLBACK_DEPARTMENTS]
  try {
    const parsed: unknown = JSON.parse(raw)
    if (
      Array.isArray(parsed) &&
      parsed.every((item) => typeof item === 'string')
    ) {
      const trimmed = parsed
        .map((item) => item.trim())
        .filter((item) => item.length > 0)
      if (trimmed.length > 0) return trimmed
    }
  } catch {
    /* 非法 JSON，回退默认清单 */
  }
  return [...FALLBACK_DEPARTMENTS]
}

async function getDepartmentListOption(): Promise<string | undefined> {
  const res = await api.get<{ success: boolean; data: SystemOption[] }>(
    '/api/option/'
  )
  return res.data.data?.find((option) => option.key === DEPARTMENT_LIST_KEY)
    ?.value
}

/** 拉取部门清单（用户页部门下拉、部门管理弹窗共用）。 */
export async function fetchDepartments(): Promise<string[]> {
  return parseDepartments(await getDepartmentListOption())
}

/** 保存部门清单。失败时抛出带后端消息的错误。 */
export async function saveDepartments(departments: string[]): Promise<void> {
  const res = await api.put<{ success: boolean; message?: string }>(
    '/api/option/',
    {
      key: DEPARTMENT_LIST_KEY,
      value: JSON.stringify(departments),
    }
  )
  if (!res.data.success) {
    throw new Error(res.data.message || 'Failed to update departments')
  }
}
