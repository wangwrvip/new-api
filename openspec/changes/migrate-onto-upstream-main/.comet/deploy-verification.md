# P5.5 部署验证与 S1–S5 结果

- 验证日期：2026-11-26
- 分支：`migrate/upstream-2026-11`，HEAD = `4b58e4ae1`（任务起点）
- 镜像：`new-api:migrate-2026-11`（image id `sha256:9b38c571c9666da2f3d6b184ad879a48bd241d4de164b81dc8f4cf8f631c4c58`，构建于 2026-11-26T08:26:11+08:00）
- 部署：`docker-compose.yml` 的 `new-api` + `postgres:15` + `redis`；`new-api` 使用本任务的最小镜像，`postgres` 数据卷为仓库 `./pg_data` 绑定挂载（`4b58e4ae1` 修复）
- 结论总览：**S1 PASS；S2 FAIL（≤10% 未达成）；S3 静态/接口检查 PASS（管理接口无 dashboard 凭据，返回 401，见限制）；S4 引用 P5.3（SQLite/PG PASS、MySQL BLOCKED）；S5 引用 P5.4 + 部署实例三维度守恒 PASS**

## 0. 部署与数据状态确认

任务开始时（事故恢复后）与结束时均确认：

| 检查项 | 结果 |
| --- | --- |
| 容器 | `new-api` (healthy)、`postgres` (up)、`redis` (up) |
| 镜像 / 挂载 | `new-api:migrate-2026-11`；`./data:/data`、`./logs:/app/logs` 绑定挂载；`postgres` 为 `./pg_data:/var/lib/postgresql/data` |
| 数据行数 | users 39、tokens 26、channels 2（验证前后一致，未改动） |
| `/api/setup` | `{"status":true,"root_init":false,...}` |
| relay token 调 `/v1/models` | HTTP 200（7 个模型） |
| Web 产物 | `web/dist/index.html`（08:22）存在；`GET /` = 200，入口 JS `GET /static/js/index.120dd10b29.js` = 200 |

`Dockerfile.local` 为最小镜像定义：`FROM debian:bookworm-slim`，仅 `COPY` 宿主机编译的 `new-api`、`LICENSE`/`NOTICE`/`THIRD-PARTY-LICENSES.md`、宿主机 CA/时区文件，以及作为 healthcheck `wget` 的 busybox；容器内不执行 `apt-get`、`go mod download` 或 `bun install`。构建上下文需先按文件头注释暂存 `p55-*` 文件。

## 1. S1 上下文限制（PASS）

### 配置

`GET /api/status` 确认服务可用后，用 SQL UPSERT 写入 `options`（键为 `context_limit_setting.*`）：

```json
{"enabled":true,"default_max_prompt_tokens":200000,
 "model_max_prompt_tokens":{"glm-5.3":500000,"deepseek-v4.1-flash":0}}
```

重启 `new-api`（保留镜像与绑定挂载）使 `config.GlobalConfig.LoadFromDB` 重新加载。

### 请求矩阵

所有请求为 OpenAI chat completions、`max_tokens:1`、非流式，使用数据库中 `status=1` 的 relay token（未打印 key）。估算 token 由上游前的 `CountRequestToken` 计算（`FamilyUnknown` 权重）。

| # | 模型 | 估计 prompt tokens | 上限 | 期望 | 实测 | 日志 id |
| --- | --- | ---: | ---: | --- | --- | ---: |
| T1 | `glm-5.3-openai`（默认） | 441826（1.5MB） | 200000 | 413 `context_limit_exceeded` | **413**，code=`context_limit_exceeded` | 25692 |
| T1b | `deepseek-v4.1-flash-openai`（默认） | 353463（1.2MB） | 200000 | 413 | **413** | 25696 |
| T2 | `glm-5.3`（覆写 500000） | ≈221000（750KB） | 500000 | 不因限制被拒 | **200**（上游 prompt_tokens 153422） | — |
| T3 | `deepseek-v4.1-flash`（覆写 0） | ≈354000（1.2MB） | 0（不限制） | 不因限制被拒 | **200**（上游 prompt_tokens 245459） | — |
| T4 | `glm-5.3-openai`（小请求） | 小 | 200000 | 放行 | **200** | — |
| T5 | `glm-5.3-openai`，`CountToken=false` | 353463（1.2MB） | 200000 | 仍 413 | **413** | 25697 |
| T5b | `glm-5.3-openai`，`CountToken=false`，小请求 | 小 | 200000 | 放行 | **200** | — |

T2/T3 返回 200 说明模型覆写生效；T1/T1b 证明默认上限仍拦截；T5 证明关闭计费估算开关（`CountToken`）后限制判定不受影响（`CountRequestToken` 独立于该开关）。为控制成本，T2/T3 均 `max_tokens:1`；T2/T3 上游实际接受并返回了 usage，未出现余额/渠道错误。

### 错误日志字段

413 分支写入 `logs`（type=5）的 `other` 含全部要求字段。示例（日志 25692）：

```json
{"context_limit":200000,"error_code":"context_limit_exceeded","error_type":"new_api_error",
 "estimated_prompt_tokens":441826,"relay_format":"openai","request_path":"/v1/chat/completions","status_code":413}
```

`estimated_prompt_tokens`、`context_limit`、`relay_format` 均在；`status_code` 为 413。

### options 恢复

进入任务时 `options` 已存在 `context_limit_setting` 行：`default_max_prompt_tokens=200000`、`model_max_prompt_tokens={"p55-big":500000,"p55-unlimited":0}`。这些是上一轮 **中断的 P5.5 attempt 留下的测试值**（fake 模型名 `p55-*`），不是生产配置；同一轮早先 08:27 写下的真实原值记录在 `/tmp/p55-options-original.tsv`：

```
default_max_prompt_tokens = 600000
enabled = true
model_max_prompt_tokens = {"deepseek-v4.1-flash":1024000,"deepseek-v4.1-flash-openai":1024000,"hy4-flash-free":1024000}
```

当前测试前值已另备份到 `/tmp/p55-context-limit-backup.txt`。验证完成后**恢复为 tsv 记录的真实原值**（而非疑似测试残留的 `p55-*`，也未 DELETE），并重启确认 `GET /api/status` = 200。恢复后配置为 default 600000、三个模型 1024000、enabled=true。

## 2. S2 估算偏差（FAIL）

方式：以 `glm-5.3-openai` 发送 5 条不同大小/含工具定义/中英文/多轮的 `chat/completions` 请求（不同大小与工具数），从 `logs.other` 读 `estimated_prompt_tokens`，与上游 `usage` 对账。实际值取统一口径 `prompt_tokens`（= `input_tokens` + `cache_tokens`，本部署上游把缓存读并入 prompt_tokens；`prompt_tokens + cache_read_tokens` 会重复计数缓存）。偏差 = (估算 − 实际) / 实际。

| 用例 | 特征 | 估算 | 上游实际 prompt | 偏差 |
| --- | --- | ---: | ---: | ---: |
| P1_small | 系统+用户消息，1 个工具 | 141 | 238 | **−40.8%** |
| P2_medium_en | ~2.5KB 英文，2 个工具 | 974 | 987 | −1.3% |
| P3_large_en | ~9KB 英文，3 个工具 | 2585 | 2445 | +5.7% |
| P4_chinese | ~4KB 中文，1 个工具 | 1435 | 1321 | +8.6% |
| P5_multiturn | ~6KB 多轮+回放工具调用，2 个工具 | 1810 | 1784（input 1528 + cache 256） | +1.5% |

补充诊断（同一实例）：

| 用例 | 特征 | 估算 | 实际 | 偏差 |
| --- | --- | ---: | ---: | ---: |
| P6_tiny_notools | 单条极短消息，无工具 | 15 | 18 | −16.7% |
| P7_tiny_tool | 单条极短消息，1 个空参数工具 | 42 | 145 | **−71.0%** |
| P8_tiny_tool_noparams | 单条极短消息，1 个无参数工具 | 28 | 128 | **−78.1%** |

**结论：FAIL。** 5 条正式样本最大绝对偏差 40.8%（P1），未达到 ≤10%；含工具的极小请求偏差可达 71%–78%。根因是上游对该代理的每个工具定义附加了约 100+ token 的固定开销，而本地估算对每个工具只加 8 token（`ToolsCount*8`）再加工具名/描述/参数文本；文本量大的请求（P2/P3/P5）因文本估算与固定开销相互抵消而落在 10% 内。该偏差是模型/上游相关的：本部署只有 glm/deepseek/hy4，无法覆盖 `FamilyClaude` 权重，故不能据此判断 Claude Code 真实流量的偏差。迁移前 6 组 Claude 权重基线最大偏差 4.6%（见 `estimate-deviation.md`），本轮结论与之并不矛盾，但 S2 的部署判定仍未通过。

## 3. S3 五项自研功能（静态/接口 PASS，附凭据限制）

管理接口（`/api/data/usage_report`、`/api/channel/glm/usage`）由 `middleware.AdminAuth()` / 权限路由保护，需要浏览器会话或 dashboard Personal Access Token。现有 relay token（`tokens` 表）只适用于 `/v1/*` 与 `/api/log/token`，直接请求管理接口返回 **401 `Unauthorized, invalid access token`**。未创建/伪造 dashboard 凭据，也未改动用户认证数据；以下以源码、数据库、构建产物和可访问的 token 日志接口为证据。**未做人工浏览器验收。**

| 功能 | 证据 |
| --- | --- |
| 使用报告（周/月对比 + 部门活跃 + PPT 导出） | 路由 `router/api-router.go:368` `GET /api/data/usage_report`（`AdminAuth`）；DTO `dto/usage_report.go` 含 `current/previous`（周/月对比）、`overview.*_prev`、`departments[]`（部门活跃率）、`users[]`、`daily[]`；前端 `web/src/features/dashboard/usage-report/` 与 `lib/export-ppt.ts`（`pptxgenjs`，`exportUsageReportPptx`）；构建产物含 `usage_report` 与 `Export PPT` 字符串 |
| 用户部门属性 | `users.department` 列存在（`varchar(64) DEFAULT '公共部门'`，索引 `idx_users_department`）；39/39 用户非空，按部门分组为 路由网关 12、平台部 7、视频部 7、单片机 5、工控机部 4、测试组 2 等；模型 `model/user.go:101` |
| GLM 渠道用量 | 路由 `router/channel-router.go:107` `GET /api/channel/glm/usage`（`ChannelRead` 权限）；处理器 `controller/glm_usage.go:198` `GetGlmChannelsUsage`（支持 `ids` 批量、并发、上限截断）；构建产物含 `glm/usage` 字符串 |
| 缓存命中率卡片/排行 | `logs.cache_read_tokens` / `logs.cache_tokens` 列存在且有数据；DTO `UsageReportUser.cache_read_tokens`，命中率 = 读 / (输入+缓存)；前端 `web/src/features/dashboard/lib/stats.ts` `cacheHitRate`、`Cache Hit Ranking`、`User Cache Hit Rate`；构建产物含对应字符串 |
| 日志请求/输出预览与估算/实际 | 已用 relay token 调 `GET /api/log/token?p=1&page_size=1000` → **200**，返回字段含 `prompt_tokens/input_tokens/cache_tokens/cache_read_tokens` 与 `other`；抽样 1000 条中 `request_preview` 995 条、`response_preview` 907 条、`estimated_prompt_tokens` 997 条、缓存维度非零 995 条；示例 `other` 含 `request_preview`、`request_preview_user`、`response_preview`、`estimated_prompt_tokens` |

UI 仅验证：构建产物存在、首页与入口 JS 返回 200、bundle 含上述功能标识字符串。未声称页面交互或浏览器人工验收。

## 4. S4 三数据库（引用 P5.3）

- SQLite：全新库与升级库 PASS（连续两次启动，新列/默认值/索引断言通过，数据保留）。
- PostgreSQL：compose 容器 PG 15.19 全新库 PASS（实际部署路径）。
- MySQL：BLOCKED（宿主 `ur` 账号无 `CREATE DATABASE` 权限，`docker pull mysql:8.2` 被环境权限分类器拒绝），新列/默认值/索引未验证。

详见 `openspec/changes/migrate-onto-upstream-main/.comet/db-verification.md` 与 `task-P5.3-report.md`。**不得声称三数据库全部兼容。**

## 5. S5 计费（引用 P5.4 + 部署抽检）

P5.4 已在 `service/text_quota_test.go` 增加 Claude / Gemini / OpenAI 固定 fixture 的三维守恒断言并通过（见 `task-P5.4-report.md`）。本轮在部署实例上各抽一条请求，核对上游 usage 与 `logs` 维度：

| 格式 | 路径 | 模型 | 上游 usage | logs 维度 (prompt/completion/input/cache/cache_read) | quota |
| --- | --- | --- | --- | --- | ---: |
| Claude | `/v1/messages` | `glm-5.3` | input 39、output 8、cache_read 0 | 39 / 8 / 39 / 0 / 0 | 500000 |
| Gemini | `/v1beta/models/deepseek-v4.1-flash:generateContent` | `deepseek-v4.1-flash` | promptTokenCount 31、candidates 8、total 39 | 31 / 8 / 31 / 0 / 0 | 100000 |
| OpenAI | `/v1/chat/completions` | `glm-5.3-openai` | prompt 39、completion 8、total 47 | 39 / 8 / 39 / 0 / 0 | 500000 |

三维度守恒成立：`input_tokens + cache_tokens = prompt_tokens`，`prompt_tokens + completion_tokens = 上游 total`。三种格式均走订阅计费（`billing_source=subscription`）。本轮只核对维度与 quota 记账值，未独立重算价格/倍率。

## 6. 产物与限制

- 提交：`Dockerfile.local` + 本文件。
- 限制与未完成项：
  1. S2 未达到 ≤10%，且未覆盖 `FamilyClaude`/`FamilyClaude47` 权重（部署无 Claude 模型）。
  2. S3 管理接口未取得 dashboard 凭据，未做浏览器人工验收；`/api/data/usage_report`、`/api/channel/glm/usage` 的运行时响应未实测，仅 401 与静态证据。
  3. S4 的 MySQL 仍未验证（继承 P5.3）。
  4. options 恢复依据 `/tmp/p55-options-original.tsv`；该文件由上一轮 attempt 在 08:27 写入，若用户认为生产原值另有来源，应按该来源再校正。
- 未提交 `/tmp` 下任何文件、`tasks.md` 或其他源码；未勾选任何 tasks 复选框。

## 7. S2 补测：大上下文（用户要求 ≥100K）

2026-11-26 追加，模型 `glm-5.3-openai`，20 个工具定义，重复英文 agent 文本，`max_tokens=1` 非流式；实际值取 `prompt_tokens`（OpenAI 格式已包含缓存命中）。

| 用例 | 请求体 | 估算 | 上游 prompt | 偏差 |
| --- | ---: | ---: | ---: | ---: |
| L1 | 660KB | 136179 | 107306 | **+26.9%** |
| L2 | 910KB | 187707 | 147684（cache 107264） | **+27.1%** |
| L3 | 1.11MB | 228930 | 179986（cache 147456） | **+27.2%** |

结论：大上下文下偏差稳定在 **+27%**，不是小请求固定开销导致；`FamilyUnknown` 权重对 GLM 文本系统性高估（约每 6.1 字符/token 的实际值被估成约 4.8 字符/token）。因此 S2 在 GLM 上仍 **FAIL**；需要为 GLM/DeepSeek 增加校准家族或调整验收口径，不能声称 ≤10% 已达成。

### 7.1 协议对比：`/v1/messages`（Anthropic）同模型复测

2026-11-26 追加，模型 `glm-5.3`，20 个 Anthropic 工具定义，同一批大文本，`/v1/messages`；实际值按 Anthropic 语义 `input_tokens + cache_read_input_tokens`。

| 用例 | 请求体 | 估算 | 实际输入 | 偏差 |
| --- | ---: | ---: | ---: | ---: |
| A1 | 660KB | 136009 | 107306 | **+26.75%** |
| A2 | 910KB | 187537 | 147684（input 40420 + cache 107264） | **+26.99%** |
| A3 | 1.11MB | 228759 | 179986（input 32530 + cache 147456） | **+27.10%** |

协议从 OpenAI 换成 Anthropic 后偏差几乎不变，说明问题不在请求格式/工具序列化，而在 GLM 文本的 tokenizer 权重。

## 8. S2 修正后复测（P5.5b）

按用户选择走 cl100k 近似路线：`tokenkit` 新增 `FamilyGLM`，GLM 文本用现有 cl100k codec；`CountRequestToken` 对 GLM 使用固定开销 `100 + 31*ToolsCount + 3*MessagesCount + 3`（所有 relay format）。非 GLM 行为不变。

镜像 `new-api:migrate-2026-11b`（含 P5.5b `a05c7165e` 与 P5.5c `27830a767`+`ab50b1045`）部署后复测：

| 用例 | 上游实际 prompt | 估算 | 偏差 |
| --- | ---: | ---: | ---: |
| 100K（660KB） | 107306 | 107311 | **+0.005%** |
| 300K（1.84MB） | 297889 | 297894 | **+0.002%** |
| 500K（3.06MB） | 494934 | 494938 | **+0.001%** |

结论：**S2 PASS**（100K–500K token 三档均 ≤0.01%）。小样本 P4 中文约 +12% 仍为已知边缘，不在本次 100K–500K 验收范围内。

## 9. 日志预览修正验证（P5.5c）

- `request_preview_user`：发送一条最后 user 消息为 `<system-reminder>…</system-reminder>\nActual question: …` 的请求，日志预览为 `Actual question: reply with the single word OK`，system-reminder 已被过滤；`CombineText` 未清洗，token 计数不受影响。
- `response_preview`：同一请求返回 `OK`，日志预览为 `OK`；reasoning/thinking 只在没有可见正文时回退。



