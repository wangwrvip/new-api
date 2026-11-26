# Gate G5 — 最终门禁证据

日期：2026-11-26
分支：`migrate/upstream-2026-11` @ `a23386635`

## 构建 / 测试

| 项目 | 结果 |
| --- | --- |
| 根模块 `go build .` | PASS |
| 根模块真实包测试（12 packages，pg_data 权限限制的替代路径） | PASS |
| `relaykit` build + test | PASS |
| `tokenkit` build + test | PASS |
| 前端 typecheck | PASS（1,543 files，0 error） |
| 前端全量测试 | PASS（256 files / 3041 tests） |
| 前端 build | PASS（`web/dist/index.html`） |
| 前端 lint | 147 errors vs `origin/main` 基线 148，无新增；改动文件定向 lint 0 error |

## 部署验证

- S1 上下文限制：PASS（新镜像 4MB 请求估算 646159 > 600000 → 413 `context_limit_exceeded`；模型覆写/0/关闭 CountToken 见 `deploy-verification.md` §1）。
- S2 估算偏差：PASS（GLM cl100k 校准后，OpenAI 协议 100K/300K/500K 偏差 +0.005%/+0.002%/+0.001%；`glm-5.3` + `/v1/messages` 偏差 0.000%/0.000%/-0.0002%）。
- S3 五项自研功能：静态/接口 PASS；管理接口 `/api/data/usage_report`、`/api/channel/glm/usage` 因无 dashboard 凭据未做浏览器人工验收，仅 401 与构建产物证据。
- S4 三库：SQLite 全新+升级 PASS；PostgreSQL 15.19（实际部署路径）全新库 PASS；MySQL 未验证，已由 owner 裁决（当前 PG 部署，后续不用 MySQL，不得声称三库兼容）。
- S5 计费：PASS（P5.4 三格式维度守恒 + 部署抽检）。

## 最终审查

- 整分支审查（opus）：无 Critical；I-1 隐私、I-3 gitignore、M-1/M-2/M-3/M-6/M-7/M-12 已在 `7b0600943` 修复并经 scoped re-review PASS。
- I-2 MySQL 未验证由 owner 裁决接受；合并说明必须保留该限制。
- S3 管理接口浏览器验收、Claude 家族 S2 实测为后续跟进项。

## 合并前状态

- 工作区干净；`git status --porcelain` 无输出。
- `git log origin/main..HEAD` 只含本 change 的主题提交（74 commits），无探索性提交。
