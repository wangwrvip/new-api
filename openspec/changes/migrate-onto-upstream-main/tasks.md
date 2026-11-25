# Tasks

## 1. 迁移准备

- [x] 1.1 从 `origin/main` 创建 `migrate/upstream-2026-11` 分支；验证 `git merge-base --is-ancestor origin/main HEAD` 返回 0
- [x] 1.2 验证 Go 1.26.10 工具链可通过 `GOTOOLCHAIN=auto` 拉取；验证根模块、`relaykit/`、`tokenkit/` 三个模块 `go build ./...` 全部通过
- [x] 1.3 记录迁移基线：`origin/main` 提交号、三模块测试基线结果、前端 `bun run test` 基线结果，写入 change 目录备注

## 2. 批次一：文档重放

- [x] 2.1 重放 CLAUDE.md 中文重写提交；冲突处以中文版为准并并入上游新增条款（如计费规则必读门禁）；验证文件内无 `<<<<<<<` 标记且引用的模块数量与实际一致
- [x] 2.2 重放 usage report 设计文档提交，丢弃内容重复的第二份；验证 `docs/` 下无同名重复文件、`git log --diff-filter=A` 只出现一次

## 3. 批次二：后端自研功能

- [x] 3.1 重放使用报告相关提交（`model/usage_report*`、`controller/usage_report.go`、`dto/usage_report.go`）；验证 `go test ./model/ ./controller/ -run UsageReport` 通过
- [x] 3.2 重放用户部门属性提交（`setting/department_setting.go`、`model/user.go`）；验证模型测试通过，且 SQLite 下 AutoMigrate 连续启动两次不报错
- [x] 3.3 重放 GLM 渠道用量提交（`controller/glm_usage.go`、路由注册、渠道前端）；验证 `go test ./controller/ -run GLM` 通过且路由注册测试覆盖新增端点
- [x] 3.4 重放缓存命中率统计提交（`cache_read_tokens` 从日志写入到使用报告聚合）；验证 `go test ./model/ -run RecordConsumeLog` 与使用报告聚合测试通过

## 4. 批次三：token 统计切换

- [x] 4.1 丢弃本地的 token 统计增强提交（`service/tokenizer.go` codec 池、`relaykit/dto` 计数增强、`service/token_estimator.go`），采用上游 `tokenkit`；验证 `cd relaykit && GOWORK=off go build ./...` 与根模块构建通过
- [x] 4.2 把上下文限制接到上游 `CountRequestToken`，保持 413 拦截与错误日志契约；验证超限请求返回 413 且错误码为 `context_limit_exceeded`
- [x] 4.3 适配消费日志的 `estimated_prompt_tokens` 写入与前端「估算 / 实际」展示；验证新增字段在日志详情接口返回中可见
- [x] 4.4 用同一批真实 Claude Code 请求复测估算偏差，与迁移前基线对比；验证偏差绝对值不超过 10%。**注：本机无夹具与配额，仅完成结构性校验，绝对偏差顺延 S2 部署验证**

## 5. 批次四：前端重建

- [ ] 5.1 以上游为基线重建 usage-logs 区域（错误日志列、估算/实际详情、请求与输出预览）；验证相关组件测试通过
- [ ] 5.2 以上游为基线重建 dashboard 区域（使用报告周/月对比、部门活跃、缓存命中率卡片与排行）；验证 `charts`、`stats` 单测与用户表测试通过
- [ ] 5.3 以上游为基线重建 system-settings 的上下文限制分组并完成路由与 section 注册；验证组件测试通过
- [ ] 5.4 重跑 `bun run i18n:sync` 并补齐自研键；验证七个语言文件的键集合一致，`bun run typecheck` 零错误

## 6. 全量验证与切换

- [ ] 6.1 三模块构建与 `make test` 全绿（根模块、`relaykit`、`tokenkit`）
- [ ] 6.2 前端 `bun run typecheck`、`bun run lint`、`bun run test` 全部通过
- [ ] 6.3 三数据库迁移验证：SQLite / MySQL / PostgreSQL 各在全新库与升级库上启动两次，确认建表幂等且既有数据与索引保留
- [ ] 6.4 计费对账：Claude / Gemini / OpenAI 三种格式的 quota 计算结果与上游实现一致
- [ ] 6.5 部署验证：构建镜像、`docker compose up -d --force-recreate new-api`，实测 S1 上下文限制、S2 估算偏差、S3 五项自研功能
- [ ] 6.6 把 `main` 快进到迁移分支并推送到 fork；验证 `git log origin/main..HEAD` 只包含预期提交

## Workflow follow-up

- WebSocket 与异步 task 路径的上下文限制覆盖作为独立 change 跟进
- 归档前确认 `backup-before-upstream` 分支与 fork 远端的保留策略
- 归档后清理 `migrate/upstream-2026-11` 分支（若已合并）
