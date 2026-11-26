# CLAUDE.md — new-api 项目约定

本文件为 Claude Code（claude.ai/code）在此仓库中工作提供指导。

## 强制：先用 Read 工具完整读取 AGENTS.md

不要把 `@AGENTS.md` 当作已加载。Claude Code 不能可靠地内联该导入。

在任何规划、编码、审查或回答项目问题之前，必须先用 Read 工具读取仓库根目录的 `AGENTS.md` 并等待完整内容返回。这是每个会话、每个新任务的第一步。

- 不要凭记忆、摘要或本文件开始工作。
- 不要因为上一轮提到过 AGENTS.md 就跳过读取。
- 不要用 grep、glob 或局部浏览代替完整读取。
- 读取后，在后续工作中遵守 `AGENTS.md` 中的每一条规则。
- 如果任务涉及 `web/`，在编辑前端文件之前还必须 Read `web/AGENTS.md`。
- 如果任务涉及计费（定义见 `AGENTS.md` 的 **Billing rules (mandatory read gate)**），在规划或修改前还必须完整 Read `.agents/rules/billing.md`；不在该定义范围内的任务可跳过。

`AGENTS.md` 是项目规则的权威来源（代码质量、JSON 封装、三数据库兼容、测试质量、前端规范、治理规则）；计费约定与安全不变量见 `.agents/rules/billing.md`。本文件只补充常用命令与架构速览，不重复其内容。

## 项目概述

AI API 网关/代理（LLM Gateway）：将 40+ 上游 AI 提供商（OpenAI、Claude、Gemini、Azure、AWS Bedrock 等）聚合到统一 API 之后，附带用户管理、计费、限流和管理后台。

- 后端：Gin、GORM v2；三个独立 Go 模块——根模块与 `tokenkit/` 使用 Go 1.26（根模块 go.mod 声明 go 1.26.10），`relaykit/` 使用 Go 1.25.1
- 前端：`web/`，React 19 + TypeScript + Rsbuild + Base UI + Tailwind，包管理器用 Bun
- 数据库：必须同时支持 SQLite、MySQL >= 5.7.8、PostgreSQL >= 9.6
- 缓存：Redis（可选）+ 内存缓存

## 常用命令

### 后端（三个独立 Go 模块）

仓库包含三个 Go 模块：根模块 `github.com/QuantumNous/new-api`，以及独立的 `relaykit/` 与 `tokenkit/` 模块。项目惯例统一使用 `GOWORK=off`。

```bash
# 构建根模块（main 包依赖 go:embed web/dist，见下方注意事项）
mkdir -p web/dist && touch web/dist/index.html   # 无前端构建产物时先创建占位文件
go build ./...
GOWORK=off go vet ./...

# 测试全部（Makefile 会跳过内嵌 web/dist 的 main 包，并分别测试三个模块）
make test

# 单个测试（标准 Go 测试）
GOWORK=off go test ./relay/helper/ -run TestXxx -v

# relaykit / tokenkit 独立验证（必须各自单独跑，根模块构建成功不代表它们可用）
(cd relaykit && GOWORK=off go build ./... && GOWORK=off go test ./...)
(cd tokenkit && GOWORK=off go build ./... && GOWORK=off go test ./...)

# 直接运行后端（开发）
go run main.go          # 默认 SQLite（one-api.db）；用 SQL_DSN/REDIS_CONN_STRING 环境变量切换
```

CI（`.github/workflows/ci.yml`）执行：根模块、relaykit 与 tokenkit 各自的 `go vet` + `go build`，随后 `make test` 覆盖三个模块；前端 `bun check -b`（与 `web/package.json` 的 `typecheck` 脚本一致）+ `bun run test`。

### 前端（在 `web/` 目录下，使用 Bun）

```bash
bun install
bun run dev           # Rsbuild 开发服务器（默认端口 3000，API 代理到 :3000；make dev-web 显式传 --port 5173）
bun run build         # 生产构建（输出 web/dist，被 Go 二进制内嵌）
bun run typecheck     # tsgo -b；每次改动 TS/TSX 后必须执行且修复到零错误
bun run lint          # oxlint；完成改动前必须修复所涉及文件的所有 error
bun run lint:fix
bun run test          # vitest run（单个文件：bun run vitest run src/path/to/x.test.ts）
bun run i18n:sync     # 同步 i18n 翻译文件
```

### 本地开发环境（makefile）

```bash
make dev              # = dev-api + dev-web
make dev-api          # docker compose -f docker-compose.dev.yml：Postgres + Redis + 后端(:3000)
make dev-api-rebuild  # 改 Go 代码后重建后端容器
make dev-web          # 前端开发服务器(:5173)
make build-web        # 生产前端构建（打真实后端二进制前必须先执行）
make reset-setup      # 重置初始化向导状态（docker Postgres 或本地 SQLite）
```

## 架构

### 请求分层

`router/ → middleware/ → controller/ → service/ → model/`

- `router/` — HTTP 路由注册，按用途分文件：`api-router.go`（管理 API）、`relay-router.go`（AI 转发）、`dashboard.go`、`web-router.go`（前端静态资源）等
- `middleware/` — 认证（auth）、渠道分发（distributor）、限流、CORS、i18n、审计等
- `controller/` — 请求处理器；转发入口是 `controller/relay.go` 的 `Relay(c, relayFormat)`
- `service/` — 业务逻辑（渠道选择、计费、任务轮询、OAuth 凭证刷新等）
- `model/` — GORM 数据模型与数据库访问；跨方言辅助在 `model/main.go`（`lockForUpdate`、`commonGroupCol`、`commonTrueVal` 等）
- `setting/` — 配置管理，按域分子包（`ratio_setting`、`operation_setting`、`system_setting`、`performance_setting`、`console_setting` 等）
- `common/` — 共享工具（JSON 封装、crypto、Redis、quota_math 等）
- `dto/`、`types/`、`constant/` — 传输对象、类型（RelayFormat 等）、常量（API 类型、渠道类型、context key）

### Relay 转发流水线（核心链路，跨多文件）

1. `router/relay-router.go` 把各路径（`/v1/chat/completions`、`/v1/messages`、Gemini、embedding、audio、image、rerank、realtime 等）绑定到 `controller.Relay(c, types.RelayFormatXxx)`
2. 中间件链完成认证与分发：`middleware/distributor.go` 根据模型/分组/权重选择渠道，写入 context
3. `relay/` 下按格式分发的 handler（如 `relay/claude_handler.go`、`relay/common_handler/`）调用 `relay.GetAdaptor(apiType)`（`relay/relay_adaptor.go`）拿到具体渠道适配器
4. `relay/channel/<provider>/` 的 Adaptor 负责该提供商的请求转换、签名、上游调用与响应转换；格式互转逻辑在 `relaykit` 模块中
5. 计费沿链路进行：预扣费（pre-consume）→ 上游 usage → 结算/退差额（`service/quota.go`、`service/text_quota.go`、`service/task_billing.go`、`service/tiered_settle.go`）

新增渠道 = 新增 `constant` 渠道/API 类型 + `relay/channel/<provider>/` 适配器 + 在 `GetAdaptor` 注册。

### relaykit 与 tokenkit 独立模块

`relaykit/` 是独立 go module（`dto/`、`types/`、`relayconvert/`、`reasonmap/`），承载中转格式转换；`tokenkit/` 是独立 go module，负责文本与图像的 token 计数与估算。两者均严禁依赖根模块，`tokenkit/` 也不得依赖 `relaykit/`；任何涉及它们的改动必须分别在对应模块目录内用 `GOWORK=off go build ./...` 单独验证。

### 前端构建被内嵌

`main.go` 通过 `go:embed web/dist` 把前端打进二进制。没有 `web/dist/index.html` 时根模块 main 包无法构建——本地开发先 `mkdir -p web/dist && touch web/dist/index.html` 占位，或先 `make build-web`。前端源码约定详见 `web/AGENTS.md`（i18n 用英文原文作键、props 不解构、TanStack Router/Query、Zustand、测试放 `__tests__/` 等）。

### 计费与配额（高风险区，改动前必读）

- 阶梯/表达式计费：先读 `pkg/billingexpr/expr.md`（设计哲学、表达式语言、归一化、配额换算），所有改动必须遵循该文档
- 配额数值运算只能用 `common/quota_math.go` 的封装（`common.QuotaFromFloat` / `QuotaRound` / `QuotaFromDecimal` 及 `*Checked` 变体），禁止裸 `int()` 转换
- 完整的计费安全不变量（乘数上界、饱和、审计）见 `.agents/rules/billing.md` 的 "Billing safety invariants"

### i18n

- 后端：`i18n/`（go-i18n，en / zh-CN / zh-TW）
- 前端：`web/src/i18n/locales/{lang}.json`，扁平 JSON，键为英文原文；组件中 `useTranslation()` + `t('English key')`

### 其他入口

- `main.go` 支持 `plugin` 子命令，运行 JS 插件 CLI（`pkg/jsplugin`）
- 异步任务（视频等）走 `router/task-router.go` + `relay/channel/task/` + `service/task_polling.go` 轮询结算
- `service/authz/` 为自研授权策略，多节点间周期同步

<comet-ambient-resume>
<!-- Managed by Comet. Edits inside this block may be replaced by comet init/update. -->
<!-- Contract: comet.resume_probe.v2 -->

## Comet Ambient Resume

在这个仓库中，开始处理需要改动或调查的任务前，如果可能存在活跃 Comet workflow，把当前用户请求传入只读探针：`comet resume-probe . --stdin --json`。

- 如果用户通过宿主明确调用任意 Comet Skill（例如 `@comet`、`/comet`、`@comet-native` 或 `/comet-hotfix`），显式调用优先于本恢复协议；不要运行 resume probe，直接进入被调用的 Skill。
- 只信任返回的 `workflow`、`skill` 和 `entrySource`；它们只由项目配置或无配置兼容回退决定。不得扫描或切换另一套 workflow。
- 如果 probe 返回 `auto_resume`，简短说明选中的 active change，并进入 `nextCommand` 指向的永久入口。不要把状态命令当作恢复入口直接推进。
- 如果 probe 返回 `ask_user`，只问一个简短问题并等待用户回复。
- 如果当前请求未明确调用 Comet Skill，且 probe 返回 `out_of_scope` 或 `none`，不要进入 Comet workflow。
- 如果配置或状态无效且没有 `nextCommand`，停止并报告原因；不要猜测另一个 workflow。
- 不能只因为存在 active change 就把无关任务挂到该 change。Native 的未提交改动由 Native 入口检查，不由探针自动归因。
</comet-ambient-resume>
