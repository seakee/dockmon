# Dockmon

[English README](./README.md)

Dockmon 是一个 Go 服务，用于采集 Docker 容器日志并写入 MySQL。
它同时支持结构化与非结构化日志，能够监听 Docker 事件，并在容器状态变化时动态启停采集器。

## 功能特性

- 采集配置中指定容器的日志。
- 解析 JSON 结构化日志，并聚合多行非结构化日志。
- 将日志及容器元数据、扩展字段落库到 MySQL。
- 监听 Docker 事件（`start`、`stop`、`die`、`destroy`）动态管理采集任务。
- 使用 Redis 保存采集断点时间戳，支持断点续采。
- 提供内部鉴权 API，用于应用注册与 JWT 令牌签发。

## 架构说明

服务启动后会初始化依赖，并并行运行三个核心组件：

- HTTP 服务（`Gin`）
- 进程内调度器（scheduler）
- Docker 日志采集器（collector）

日志采集主流程：

1. 将配置中的容器名解析为容器 ID。
2. 通过 Docker SDK 建立日志流（`Follow + Timestamps`）。
3. 逐行解析：
   - JSON 日志：映射已知字段（`L`、`T`、`C`、`M`、`TraceID`），未知字段放入 `extra`。
   - 非结构化日志：按多行缓冲聚合并推断日志级别。
4. 清洗消息内容（移除 ANSI 控制序列、不可打印字符、非法 UTF-8，并安全截断）。
5. 写入 MySQL `log` 表。
6. 将最新时间戳写入 Redis 作为增量断点。

## 目录结构

```text
.
├── app/
│   ├── http/                # API 处理器、中间件、路由
│   ├── monitor/             # Docker 监听与日志解析
│   ├── job/                 # 调度任务
│   ├── model/               # GORM 模型
│   ├── repository/          # 数据访问层
│   ├── service/             # 业务服务层
│   └── pkg/                 # 通用包（jwt、trace、schedule、错误码）
├── bootstrap/               # 应用引导与组件启动
├── bin/
│   ├── configs/             # local/dev/prod 配置
│   ├── data/sql/            # 建表与迁移 SQL
│   └── lang/                # i18n 语言包
├── scripts/                 # 辅助脚本
├── Dockerfile
├── Makefile
└── main.go
```

## 环境要求

- Go `1.24+`（以 `go.mod` 为准）
- Docker Daemon（Dockmon 通过 Docker socket/API 工作）
- MySQL 8+
- Redis

## 快速开始（本地）

1. 克隆仓库。

```bash
git clone https://github.com/seakee/dockmon.git
cd dockmon
```

2. 创建本地配置并填写必填项。

```bash
cp bin/configs/local.json.default bin/configs/local.json
```

必须配置：

- `system.jwt_secret`（必填，建议至少 32 位随机字符串）
- `databases[0]` 的数据库连接参数
- `redis[0]` 的连接参数
- `collector.container_name`（要监控的容器名列表）

3. 初始化数据库。

```bash
mysql -u <user> -p <database> < bin/data/sql/auth_app.sql
mysql -u <user> -p <database> < bin/data/sql/log.sql
```

如果是历史部署升级，额外执行：

```bash
mysql -u <user> -p <database> < bin/data/sql/migration/20260226_log_utf8mb4_fix.sql
```

4. 编译并运行。

```bash
make build
RUN_ENV=local make run
```

等效脚本命令：

```bash
./scripts/dockmon.sh build
RUN_ENV=local ./scripts/dockmon.sh run
```

## Docker 运行

构建镜像：

```bash
make docker-build
```

运行容器：

```bash
RUN_ENV=local make docker-run
```

手动运行示例：

```bash
docker run -d --name dockmon \
  -p 8085:8080 \
  -v /var/run/docker.sock:/var/run/docker.sock \
  -v "$(pwd)/bin/configs":/bin/configs \
  -e APP_NAME=dockmon \
  -e RUN_ENV=local \
  dockmon:latest
```

## 配置说明

配置文件路径规则：`bin/configs/<RUN_ENV>.json`。

环境变量：

- `RUN_ENV`：选择配置文件（`local`、`dev`、`prod`），默认 `local`。
- `APP_NAME`：运行时覆盖 `system.name`。

关键配置块：

- `system`：运行模式、端口、JWT、语言。
- `databases`：MySQL 配置与重试策略。
- `redis`：Redis 客户端配置（Dockmon 依赖名为 `dockmon` 的实例）。
- `collector`：
  - `monitor_self`：在容器内运行时自动追加当前服务容器。
  - `container_name`：监控容器名列表。
  - `time_layout`：可解析时间格式。
  - `unstructured_log_line_flags`：识别非结构化日志起始行前缀。

## HTTP API

基础路由组：`/dockmon`

健康检查接口：

- `GET /dockmon/internal/ping`
- `GET /dockmon/internal/admin/ping`
- `GET /dockmon/internal/service/ping`
- `GET /dockmon/external/ping`
- `GET /dockmon/external/app/ping`
- `GET /dockmon/external/service/ping`

鉴权接口：

- `POST /dockmon/internal/service/server/auth/token`
  - 请求类型：`application/x-www-form-urlencoded`
  - 参数：`app_id`、`app_secret`
  - 返回：JWT 令牌与 `expires_in`

- `POST /dockmon/internal/service/server/auth/app`
  - 需要 `Authorization` 请求头（原始 JWT 字符串，不是 Bearer 包装）
  - 请求类型：JSON
  - 参数：`app_name`、`description`、`redirect_uri`
  - 返回：生成的 `app_id` 与 `app_secret`

获取令牌示例：

```bash
curl -X POST "http://127.0.0.1:8080/dockmon/internal/service/server/auth/token" \
  -H "Content-Type: application/x-www-form-urlencoded" \
  -d "app_id=<app_id>&app_secret=<app_secret>"
```

创建应用示例：

```bash
curl -X POST "http://127.0.0.1:8080/dockmon/internal/service/server/auth/app" \
  -H "Authorization: <jwt_token>" \
  -H "Content-Type: application/json" \
  -d '{
    "app_name": "my-service",
    "description": "internal service",
    "redirect_uri": "https://example.com/callback"
  }'
```

## 测试

运行全部测试：

```bash
go test ./...
```

仅运行 monitor 包测试：

```bash
go test ./app/monitor -v
```

## 运行说明与排障

- 必须挂载 Docker Socket：`/var/run/docker.sock`。
- collector 依赖 Redis 存储采集断点和调度锁。
- 调用鉴权接口前需先存在 `auth_app` 表。
- 配置示例默认 `stdout` 日志。若切换文件日志，请确认配置键与代码期望一致（`log_path`）。

## 许可证

MIT License，见 [LICENSE](./LICENSE)。
