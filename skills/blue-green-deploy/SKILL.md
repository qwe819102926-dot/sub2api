---
name: blue-green-deploy
description: Docker 应用零停机蓝绿发布、健康检查与自动回滚。用于本项目的 staging 或生产 Docker 应用更新；不用于数据库、Redis 或持久化数据迁移。
---

# 蓝绿部署

为本 Sub2API 项目执行单机 Docker 蓝绿发布。当前方案使用现有 PostgreSQL、Redis
和 `/app/data`，只替换应用容器，并通过 Caddy 在 blue/green 端口之间切换流量。

## 授权边界

- 用户只要求方案或审查时，不修改仓库、不连接服务器、不改变生产流量。
- 代码修改和本地测试可以直接执行；推送 Git、发布镜像、连接服务器或切换生产流量前，必须取得对应的明确授权。
- 不在服务器上编辑应用源代码或构建镜像；镜像由 GitHub Actions 构建并发布。
- 不启动、重建或删除 PostgreSQL、Redis、数据卷、`.env`、`data/`、`postgres_data/` 或 `redis_data/`。

## 工作目录定位

调用 skill 时不要假设当前目录就是仓库根目录。先定位 Sub2API 的 Git 根目录：

1. 优先从当前目录及其父目录执行 `git rev-parse --show-toplevel`。
2. 如果当前目录不是 Git 仓库，检查当前目录的直接子目录；选择同时包含
   `deploy/blue-green-update.sh`、`backend/` 和 `frontend/` 的唯一目录。
3. 后续仓库检查、文件读取和本地测试都以找到的 Git 根目录为基准。不要因为外层
   工作区包含多个项目就报告“目录不对”。
4. 服务器上的部署目录（例如 `/opt/sub2api-deploy`）与本地 Git 根目录是两个不同
   概念；只有实际执行服务器发布脚本时才切换到部署目录。

## 标准流程

1. 检查仓库状态和当前分支，保留无关的用户修改。
2. 在 `dev` 或其他非 `main` 分支运行现有 CI 冒烟测试，并在 staging 验证。
3. 合并到 `main` 后等待 `Build and publish Docker image` 成功；生产部署使用 `sha-<commit>` 不可变镜像 tag，不依赖 `latest`。
4. 在服务器部署目录运行 `deploy/blue-green-update.sh`：
   - 从当前应用容器复制环境变量、网络和所有挂载；
   - 在 `127.0.0.1:18080` 启动 green，不创建第二套数据库或 Redis；
   - 检查 green `/health`，校验并 reload Caddy；
   - 检查公网 `/health`，成功后更新 `.env` 的 `SUB2API_IMAGE`；
   - 默认保留旧容器 120 秒，再优雅停止；长连接较多时调大 `--drain-seconds`。
5. 切换失败时恢复 Caddy、`.env`、active 状态文件并启动旧容器；成功后报告镜像、容器、健康响应、备份目录和旧容器排空结果。

## 多实例安全

应用已经提供 Redis leader lock，并在 Redis 不可用时回退到 PostgreSQL advisory
lock。使用蓝绿部署时要确认新增的周期任务也采用该机制；不要通过启动第二套
PostgreSQL 或 Redis 来“隔离” green。

`SERVER_SHUTDOWN_TIMEOUT` 默认是 `120s`，控制收到 SIGTERM 后在途 HTTP/SSE 请求的
优雅关闭窗口。若调整 `--drain-seconds`，同时考虑该环境变量和 Docker stop 超时。

## 参考文件

- 详细服务器前置条件、发布命令和手动回滚：`deploy/BLUE_GREEN.md`
- 实际执行入口：`deploy/blue-green-update.sh`
- 生产 Compose 镜像变量：`deploy/docker-compose.yml` 中的 `SUB2API_IMAGE`
- graceful shutdown 实现：`backend/cmd/server/main.go`
