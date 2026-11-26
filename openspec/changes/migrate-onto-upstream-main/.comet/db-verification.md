# P5.3 三数据库迁移验证记录

- 验证日期：2026-11-25（SQLite）、2026-11-26（PostgreSQL 容器补测）
- 当前分支：`migrate/upstream-2026-11`；二进制来自 HEAD `a0bda1fc74bd06556965db04abb97f7c9e1f31b3`；备份分支 `backup-before-upstream` = `0f9d13e9a129527a26a5b5b6ce27adf8d6c06723`
- 结果总览：**SQLite PASS（全新库 + 升级库）；PostgreSQL PASS（compose 容器 15.19，全新库）；MySQL BLOCKED**。三库矩阵仍未全绿，不能据此声称三数据库全部兼容；但当前部署使用的 PostgreSQL（docker-compose.yml:29）已实测通过。
- 全程未使用、未修改、未删除 `ur_verify`；未执行任何 `DROP DATABASE`；未触碰 compose 中的 `new-api` / `new-api-log` 两库。所有验证目标均为全新库或全新文件，名称带 `p53` 后缀。

## 环境

```bash
export PATH=/usr/local/go/bin:$HOME/.bun/bin:$HOME/.nvm/versions/node/v24.18.0/bin:$PATH
```

| 项目 | 版本 / 命令 |
| --- | --- |
| Go | `go version go1.26.10 linux/amd64` |
| SQLite | `sqlite3 --version` → `3.45.1 2024-01-30 16:01:20 e876e51a0ed5c5b3126f52e532044363a014bc594cfefa87ffb5b82257ccalt1 (64-bit)` |
| MySQL 客户端（宿主） | `mysql Ver 8.0.46-0ubuntu0.24.04.3 for Linux on x86_64 ((Ubuntu))` |
| MySQL 服务端（宿主） | `SELECT VERSION()` → `8.0.46-0ubuntu0.24.04.3`（账号 `ur` 无建库权限，未用于验证） |
| PostgreSQL 客户端（宿主） | `psql (PostgreSQL) 16.15 (Ubuntu 16.15-0ubuntu0.24.04.1)`（宿主原生 5432 是另一套实例，未用于验证） |
| PostgreSQL 服务端（compose 容器） | `docker compose exec -T postgres psql -U root -d postgres -Atc 'SHOW server_version;'` → `15.19 (Debian 15.19-1.pgdg13+2)` |

二进制构建（在仓库根目录，前端 `web/dist/index.html` 已存在）：

```bash
GOWORK=off go build -o /tmp/p53-new-api .        # rc=0
git archive backup-before-upstream | tar -x -C /tmp/p53-old-src
(cd /tmp/p53-old-src && mkdir -p web/dist && printf '<!doctype html>\n' > web/dist/index.html && GOWORK=off go build -o /tmp/p53-new-api-old .)   # rc=0
```

按任务约束使用 `git archive` 而非 `git worktree add`。目标列清单（来自 ledger Gate G2）：`logs.input_tokens`、`logs.cache_tokens`、`logs.cache_read_tokens`、`users.department`、`quota_data.input_tokens`、`quota_data.output_tokens`、`quota_data.cache_tokens`。ClickHouse 不在本任务矩阵内。

## 1. SQLite 全新库 —— PASS

库文件：`/tmp/migrate_p53_fresh.db`（验证前不存在）。

```bash
PORT=3011 SQLITE_PATH=/tmp/migrate_p53_fresh.db DEBUG=true timeout 30s /tmp/p53-new-api > /tmp/p53-sqlite-fresh-run1.log 2>&1   # rc=124
PORT=3011 SQLITE_PATH=/tmp/migrate_p53_fresh.db DEBUG=true timeout 30s /tmp/p53-new-api > /tmp/p53-sqlite-fresh-run2.log 2>&1   # rc=124
```

- 两次 `rc=124`：`timeout` 在 30s 后发信号，日志末尾均为 `received signal: terminated, shutting down...` / `server exited`，属预期的优雅关闭。
- `DEBUG=true` 输出 GORM SQL；`grep -ic 'ALTER TABLE'` 两次均为 **0**；无 AutoMigrate 错误、无 panic/fatal。

表结构断言（`sqlite3 /tmp/migrate_p53_fresh.db`）：

```
PRAGMA table_info(logs):
11|input_tokens|INTEGER|0|0|0
12|cache_tokens|INTEGER|0|0|0
13|cache_read_tokens|INTEGER|0|0|0

PRAGMA table_info(users):
18|department|varchar(64)|0|"公共部门"|0

PRAGMA index_list(users):
12|idx_users_department|0|c|0

PRAGMA table_info(quota_data):
10|input_tokens|INTEGER|0|0|0
11|output_tokens|INTEGER|0|0|0
12|cache_tokens|INTEGER|0|0|0
```

`.schema` 摘录：`logs` 含 `` `input_tokens` integer DEFAULT 0 ``、`` `cache_tokens` integer DEFAULT 0 ``、`` `cache_read_tokens` integer DEFAULT 0 ``；`users` 含 `` `department` varchar(64) DEFAULT "公共部门" `` 及 `` CREATE INDEX `idx_users_department` ON `users`(`department`) ``；`quota_data` 含三个新整数列（`DEFAULT 0`）。

## 2. SQLite 升级库 —— PASS

升级库：`/tmp/migrate_p53_upgrade.db`（验证前不存在）。

```bash
PORT=3012 SQLITE_PATH=/tmp/migrate_p53_upgrade.db timeout 15s /tmp/p53-new-api-old > /tmp/p53-sqlite-upgrade-old.log 2>&1   # rc=124
```

旧二进制建库后写入代表数据（直接 SQL，用户/渠道/日志/统计各一行）：

```sql
INSERT INTO users (id, username, "password", display_name, role, status, email, quota, used_quota, request_count, "group", department, aff_code)
  VALUES (7, 'p53_upgrade_user', 'p53hash', 'P53 Upgrade', 1, 1, 'p53@example.invalid', 123456, 789, 3, 'default', '工程部', 'p53aff');
INSERT INTO channels (id, type, key, status, name, weight, models, "group", priority)
  VALUES (11, 1, 'p53-key', 1, 'p53 channel', 1, 'gpt-4o', 'default', 0);
INSERT INTO logs (id, user_id, created_at, type, content, username, token_name, model_name, quota,
                  prompt_tokens, completion_tokens, input_tokens, cache_tokens, cache_read_tokens,
                  channel_id, channel_name, "group", request_id)
  VALUES (21, 7, 1795600000, 1, 'p53 representative log', 'p53_upgrade_user', 'p53-token',
          'gpt-4o', 100, 50, 20, 40, 5, 3, 11, 'p53 channel', 'default', 'p53-req-1');
INSERT INTO quota_data (id, user_id, username, model_name, created_at, use_group, token_id, channel_id,
                        input_tokens, output_tokens, cache_tokens, cache_read_tokens, count, quota)
  VALUES (5, 7, 'p53_upgrade_user', 'gpt-4o', 1795600000, 'default', 0, 11, 40, 20, 5, 3, 1, 100);
```

然后用新二进制连续启动两次：

```bash
PORT=3012 SQLITE_PATH=/tmp/migrate_p53_upgrade.db DEBUG=true timeout 30s /tmp/p53-new-api > /tmp/p53-sqlite-upgrade-run1.log 2>&1   # rc=124
PORT=3012 SQLITE_PATH=/tmp/migrate_p53_upgrade.db DEBUG=true timeout 30s /tmp/p53-new-api > /tmp/p53-sqlite-upgrade-run2.log 2>&1   # rc=124
```

- 两次均优雅退出（日志 `server exited`），无 AutoMigrate 错误、无 panic/fatal。
- 第一次启动有 5 条成功的 `ALTER TABLE ... ADD`（均非本次 7 个目标列，而是备份版本之后新增的其他列）：`tokens.allow_user_quota_query`、`users.access_token_created_at`、`passkey_credentials.rp_id`、`quota_data.cache_creation_tokens DEFAULT 0`、`task_plugins.icon`。第二次启动 `ALTER TABLE` 计数为 **0**，证明重启幂等。
- 说明：`backup-before-upstream` 的 `users`、`logs`、`quota_data` 已含本次 7 个目标列，因此升级路径并不重新添加这些列；本验证覆盖的是「旧版本库 → 当前版本连续两次启动」的数据与索引保留，目标列存在性在升级后逐项检查。

数据保留断言（升级前 = 升级后，逐项核对）：

| 断言 | 升级前 | 两次新二进制启动后 |
| --- | --- | --- |
| users / channels / logs / quota_data 行数 | 1 / 1 / 1 / 1 | 1 / 1 / 1 / 1 |
| user id=7 | `p53_upgrade_user\|P53 Upgrade\|1\|1\|123456\|789\|3\|default\|工程部` | 完全相同（含自定义 department `工程部`） |
| channel id=11 | `1\|p53 channel\|1\|gpt-4o` | 完全相同 |
| log id=21 | `user=7, model=gpt-4o, quota=100, prompt=50, completion=20, input=40, cache=5, cache_read=3, request_id=p53-req-1` | 完全相同 |
| quota_data id=5 | `user=7, model=gpt-4o, input=40, output=20, cache=5, quota=100` | 完全相同 |

索引保留断言：`sqlite_master` 中 `users/channels/logs/quota_data` 四表的索引清单在升级前后各 39 行，`diff` 输出为空（含 `idx_users_department`）。升级后目标列再次核对：`logs` 的 `input_tokens/cache_tokens/cache_read_tokens`、`users.department`（默认值 `"公共部门"`、`idx_users_department` 存在）、`quota_data` 的 `input_tokens/output_tokens/cache_tokens` 均在。

## 3. PostgreSQL（compose 容器，实际部署所用）—— PASS

宿主原生 PG 16.15 的 `ur` 账号无 `CREATEDB`（`ur|f|f|f`，`CREATE DATABASE migrate_p53_pg` → `permission denied to create database`），sudo 需口令；按账本裁决不碰该实例上的 `ur_verify`。按协调员澄清改部署实际使用的 compose `postgres` 容器（`docker-compose.yml` service `postgres`，image `postgres:15`，容器未映射宿主端口，superuser 为 `root`）。

库创建（先查存在性，目标名未被占用才创建；未 DROP 任何库）：

```bash
docker compose exec -T postgres psql -U root -d postgres -Atc "SELECT datname FROM pg_database WHERE datistemplate = false ORDER BY datname;"
# new-api / postgres → migrate_p53_pg2 不存在
docker compose exec -T postgres psql -U root -d postgres -c 'CREATE DATABASE migrate_p53_pg2;'   # CREATE DATABASE，rc=0
docker compose exec -T postgres psql -U root -d postgres -Atc 'SHOW server_version;'
# 15.19 (Debian 15.19-1.pgdg13+2)
```

新二进制连续启动两次：把宿主编译的二进制拷入容器后，用容器内 `timeout` 保证优雅终止（DSN 中的口令来自 compose 的 `POSTGRES_PASSWORD`）：

```bash
docker cp /tmp/p53-new-api postgres:/tmp/p53-new-api                                            # rc=0
docker compose exec -T -e PORT=3013 -e DEBUG=true \
  -e SQL_DSN='postgresql://root:123456@127.0.0.1:5432/migrate_p53_pg2?sslmode=disable' \
  postgres timeout 30s /tmp/p53-new-api > /tmp/p53-pg-run1.log 2>&1                             # rc=124
docker compose exec -T -e PORT=3013 -e DEBUG=true \
  -e SQL_DSN='postgresql://root:123456@127.0.0.1:5432/migrate_p53_pg2?sslmode=disable' \
  postgres timeout 30s /tmp/p53-new-api > /tmp/p53-pg-run2.log 2>&1                             # rc=124
docker compose exec -T postgres rm -f /tmp/p53-new-api                                          # rc=0（清理）
```

- 日志显示 `[SYS] using PostgreSQL as database`、`database migration started`，两次末尾均为 `received signal: terminated` / `server exited`；无 `ERROR:` / `FATAL:` / panic。
- `grep -ic 'ALTER TABLE'` 两次均为 **0**，全新库迁移幂等。

表结构断言（`psql -U root -d migrate_p53_pg2`）：

```
 table_name |    column_name    |     data_type     |        column_default
 logs       | cache_read_tokens | bigint            | 0
 logs       | cache_tokens      | bigint            | 0
 logs       | input_tokens      | bigint            | 0
 quota_data | cache_tokens      | bigint            | 0
 quota_data | input_tokens      | bigint            | 0
 quota_data | output_tokens     | bigint            | 0
 users      | department        | character varying | '公共部门'::character varying
```

`\d logs` 摘录：`input_tokens / cache_tokens / cache_read_tokens` 三列均为 `bigint`，默认值 `0`，主键 `id` 使用 `nextval('logs_id_seq'::regclass)` 序列。索引断言：

```
 idx_users_department | CREATE INDEX idx_users_department ON public.users USING btree (department)
```

## 4. MySQL 8.0.46 —— BLOCKED（未验证）

按"只用全新库、若已存在则换名、不 DROP 既有库"的约束，先尝试创建 `migrate_p53_mysql`：

```bash
mysql -h127.0.0.1 -u ur -purpass -e 'SHOW DATABASES'
# 仅 information_schema / performance_schema / ur_verify → 目标库不存在
mysql -h127.0.0.1 -u ur -purpass -e 'SHOW GRANTS FOR CURRENT_USER();'
# GRANT USAGE ON *.* TO `ur`@`127.0.0.1`; GRANT ALL PRIVILEGES ON `ur_verify`.* TO `ur`@`127.0.0.1`
mysql -h127.0.0.1 -u ur -purpass -e 'CREATE DATABASE migrate_p53_mysql CHARACTER SET utf8mb4;'
# ERROR 1044 (42000): Access denied for user 'ur'@'127.0.0.1' to database 'migrate_p53_mysql'
sudo -n mysql -e 'SELECT VERSION(); SHOW DATABASES;'
# sudo: a password is required
```

随后按协调员建议尝试 `docker pull mysql:8.2` 以临时容器建全新库，但该操作被执行环境的权限分类器拒绝（Remote Shell Writes，未获得对共享/远程工作负载写入的显式授权）。未尝试其它镜像源或凭据发现方式。

结论：`ur` 账号仅对禁止使用的 `ur_verify` 有权限；sudo 需口令；docker 镜像拉取被权限拒绝。未创建库、未启动新二进制、未执行 `SHOW COLUMNS FROM logs/users` / `SHOW INDEX FROM users` 等断言。`logs` 三列、`users.department` 默认值 `公共部门`、department 索引及 `quota_data` 三列在 MySQL 上**未验证**，不作通过断言。compose 中 mysql 服务本身是注释状态（`docker-compose.yml:92-104`），不影响当前部署结论。

## 结论与阻塞

| 数据库 | 全新库连续两次启动 | 升级库 | 结论 |
| --- | --- | --- | --- |
| SQLite 3.45.1 | PASS（0 次 ALTER） | PASS（第一次 5 次 ALTER，第二次 0 次；数据/索引保留） | PASS |
| PostgreSQL 15.19（compose 容器，实际部署） | PASS（0 次 ALTER；新列/默认值/索引断言通过） | 未执行（brief 的升级库场景仅要求 SQLite） | 全新库 PASS |
| MySQL 8.0.46（宿主原生） | BLOCKED：`ur` 无 CREATE DATABASE 权限、sudo 需口令、docker pull 被权限拒绝 | 未执行 | 未验证 |

阻塞解除条件：允许拉取 `mysql:8.2` 并由临时容器提供全新库（或管理员预建空库并授权），再按 brief Step 3 执行两次启动与列/默认值/索引断言。在此之前不能声称三数据库全部验证通过；实际部署使用的 PostgreSQL 路径已验证通过。

## Owner 裁决（2026-11-26）

部署方确认当前及可预见的生产环境均使用 PostgreSQL，MySQL 不测试、后续不会使用。因此：

- 本 change 只保证 SQLite 与 PostgreSQL 的验证结果；
- **不得对外声称三数据库全部兼容**；
- 将来若启用 MySQL，必须补跑全新库 + 升级库 + 连续两次启动的矩阵，重点验证 `users.department` 的中文 DEFAULT 在非 utf8mb4 历史库上的行为。
