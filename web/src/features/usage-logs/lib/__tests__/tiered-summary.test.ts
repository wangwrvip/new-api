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
import { describe, expect, test } from 'vitest'

import {
  parseTiersFromExpr,
  splitBillingExprAndRequestRules,
} from '@/features/pricing/lib/billing-expr'

import type { LogOtherData } from '../../types'
import { getTieredBillingSummary } from '../format'

function b64(expr: string): string {
  return Buffer.from(expr, 'utf8').toString('base64')
}

// Peak-hour flat-fee expression observed on a real deployment: the tier body is
// a pure constant (per-request fee), the 14:00-18:00 rule multiplies it by 3.
const PEAK_HOUR_FLAT_EXPR =
  '(tier("base", 1000000)) * ((hour("Asia/Shanghai") >= 14 && hour("Asia/Shanghai") < 18) ? 3 : 1)'

const TOKEN_PRICED_EXPR = 'tier("base", p * 2.5 + c * 15 + cr * 0.25)'

function flatOther(expr: string, matchedTier: string): LogOtherData {
  return {
    billing_mode: 'tiered_expr',
    expr_b64: b64(expr),
    matched_tier: matchedTier,
    cache_tokens: 134912,
    group_ratio: 1,
    user_group_ratio: -1,
  }
}

function requireSummary(other: LogOtherData) {
  const summary = getTieredBillingSummary(other)
  expect(summary).not.toBeNull()
  if (!summary) throw new Error('expected a tiered billing summary')
  return summary
}

describe('parseTiersFromExpr flat constant detection', () => {
  test('captures a pure-numeric tier body as flatConstant', () => {
    // The log summary parses the billing half after request-rule factors are
    // split off, matching the upstream parseTiersFromExpr contract.
    const tiers = parseTiersFromExpr(
      splitBillingExprAndRequestRules(PEAK_HOUR_FLAT_EXPR).billingExpr
    )
    expect(tiers).toHaveLength(1)
    expect(tiers[0].label).toBe('base')
    expect(tiers[0].flatConstant).toBe(1000000)
  })

  test('leaves flatConstant unset for token-priced tier bodies', () => {
    const tiers = parseTiersFromExpr(TOKEN_PRICED_EXPR)
    expect(tiers).toHaveLength(1)
    expect(tiers[0].flatConstant).toBeUndefined()
  })
})

describe('getTieredBillingSummary flat per-call fee', () => {
  test('exposes flatPerCallUsd for a flat constant expression', () => {
    const summary = requireSummary(flatOther(PEAK_HOUR_FLAT_EXPR, 'base'))
    expect(summary.priceEntries).toHaveLength(0)
    expect(summary.flatPerCallUsd).toBe(1)
  })

  test('keeps flatPerCallUsd unset for token-priced expressions', () => {
    const summary = requireSummary(flatOther(TOKEN_PRICED_EXPR, 'base'))
    expect(summary.priceEntries.length).toBeGreaterThan(0)
    expect(summary.flatPerCallUsd).toBeUndefined()
  })

  test('returns null for non-tiered payloads', () => {
    expect(getTieredBillingSummary({ model_price: 1 })).toBeNull()
  })
})
