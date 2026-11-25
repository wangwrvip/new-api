# Gate G4 — 前端门禁证据

日期：2026-11-25
分支：`migrate/upstream-2026-11` @ `9476677d2`

| 项目 | 结果 | 证据 |
| --- | --- | --- |
| `bunx bun@1.4.4 run typecheck` | PASS | `✓ No type errors in 1,542 files across 2 projects [698.85ms]` |
| `bunx bun@1.4.4 run test` | PASS | `Test Files 255 passed (255)`，`Tests 3031 passed (3031)`，222.76s |
| 全项目 lint | 无新增 error | `origin/main` 基线 `git archive` + 同一配置实测 148 error；当前 147 error；差异来自本分支修复，非新增回归 |
| 改动文件定向 lint | PASS | `bunx oxlint -c .oxlintrc.json <origin/main...HEAD 的 web/src TS/TSX>` exit=0，`0 error` |
| `bunx bun@1.4.4 run build` | PASS | 退出 0，`web/dist/index.html` 生成（1683 bytes） |
| 七语言键集合 | PASS | 七份各 7872 键，SHA256 统一 `c31e2956be4ecced675522fc6d04b99250c8dc5de28bbecbb2481e89c8426946` |

裁定：计划 Gate G4 的「`bun run lint` 零 error」在上游基线即不成立（148 error），按 ledger 裁决改为「相对基线零新增 + 改动文件定向零 error」。未修复的 147 个 error 全部位于本 change 未修改的上游文件，如实记录，不得声称全项目零 error。

顺延项：
- G3 绝对估算偏差 ≤10% 顺延 S2 部署验证。
- P4.6 Minor：`user-range.ts` 6 个动态 labelKey 未登记 `static-keys.ts`（无运行时影响）。
- P4.5 三条 Minor（旧值警告不阻止保存、弹窗草稿被 refetch 覆盖、非 2xx 错误显示 Axios 原文）。
- P4.4 四条 Minor（见 ledger）。
