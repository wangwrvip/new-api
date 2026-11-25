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
import type { TFunction } from 'i18next'
import { describe, expect, it, vi } from 'vitest'

import {
  DASHBOARD_ADMIN_ONLY_SECTIONS,
  DASHBOARD_SECTION_IDS,
  getDashboardSectionNavItems,
} from '../section-registry'

type CapturedSection = {
  id: string
  titleKey: string
  adminOnly?: boolean
  build: () => null
}

type CapturedConfig = {
  sections: readonly CapturedSection[]
  defaultSection: string
  basePath: string
  urlStyle: 'path'
}

// Capture the definitions handed to the shared registry without changing the
// production module: the adminOnly flag itself is not part of the nav output.
const { capturedConfigs } = vi.hoisted(() => ({
  capturedConfigs: [] as CapturedConfig[],
}))

vi.mock('@/features/system-settings/utils/section-registry', async () => {
  const actual = await vi.importActual<
    typeof import('@/features/system-settings/utils/section-registry')
  >('@/features/system-settings/utils/section-registry')
  return {
    ...actual,
    createSectionRegistry: (config: CapturedConfig) => {
      capturedConfigs.push(config)
      return actual.createSectionRegistry(
        config as unknown as Parameters<typeof actual.createSectionRegistry>[0]
      )
    },
  }
})

const t = ((key: string) => key) as TFunction

describe('dashboard section registration', () => {
  it('registers usage-report as admin-only while keeping the upstream sections', () => {
    expect(capturedConfigs).toHaveLength(1)
    const sections = capturedConfigs[0].sections
    expect(sections.map((section) => section.id)).toEqual([
      'overview',
      'models',
      'flow',
      'users',
      'usage-report',
    ])
    expect(
      sections.find((section) => section.id === 'usage-report')?.adminOnly
    ).toBe(true)
    expect(DASHBOARD_SECTION_IDS).toContain('usage-report')
    expect(DASHBOARD_ADMIN_ONLY_SECTIONS.has('usage-report')).toBe(true)
  })

  it('shows the admin-only usage-report nav item only to admins', () => {
    const adminUrls = getDashboardSectionNavItems(t, {
      isAdmin: true,
    }).map((item) => item.url)
    expect(adminUrls).toEqual([
      '/dashboard/overview',
      '/dashboard/models',
      '/dashboard/flow',
      '/dashboard/users',
      '/dashboard/usage-report',
    ])

    const nonAdminUrls = getDashboardSectionNavItems(t, {
      isAdmin: false,
    }).map((item) => item.url)
    expect(nonAdminUrls).toEqual([
      '/dashboard/overview',
      '/dashboard/models',
      '/dashboard/flow',
    ])
  })
})
