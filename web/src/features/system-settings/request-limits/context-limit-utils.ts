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
export type ModelLimitMap = Record<string, number>

export function isValidModelLimitText(value: string): boolean {
  return value
    .split('\n')
    .every((line) => line.trim() === '' || /^\s*\S+\s*=\s*\d+\s*$/.test(line))
}

export function parseModelLimitLines(value: string): ModelLimitMap {
  const map: ModelLimitMap = {}
  for (const line of value.split('\n')) {
    const trimmed = line.trim()
    if (trimmed === '') continue
    const eq = trimmed.indexOf('=')
    if (eq <= 0) continue
    const name = trimmed.slice(0, eq).trim()
    const limit = Number.parseInt(trimmed.slice(eq + 1).trim(), 10)
    if (name === '' || !Number.isInteger(limit) || limit < 0) continue
    map[name] = limit
  }
  return map
}

export function formatModelLimitLines(map: ModelLimitMap): string {
  return Object.keys(map)
    .sort()
    .map((name) => `${name}=${map[name]}`)
    .join('\n')
}

export function canonicalModelLimitJson(map: ModelLimitMap): string {
  const sorted: ModelLimitMap = {}
  for (const name of Object.keys(map).sort()) {
    sorted[name] = map[name]
  }
  return JSON.stringify(sorted)
}
