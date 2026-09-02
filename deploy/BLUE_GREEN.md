# Docker 蓝绿部署

本项目使用单机 Docker + Caddy 实现应用容器的蓝绿切换。PostgreSQL、Redis
和应用数据卷始终由现有 Compose 栈管理，部署脚本只创建/停止应用容器。

## 前置条件

- 生产机器安装 Docker、Docker Compose、Caddy、Python 3 和 `flock`。
- 当前应用容器名为 `sub2api`，对外监听 `127.0.0.1:8080`。
- Caddyfile 中存在唯一的 `localhost:8080` 或 `127.0.0.1:8080` upstream。
- `deploy/.env` 中设置固定的 `JWT_SECRET`、`TOTP_ENCRYPTION_KEY` 和数据库密码。
- 镜像使用不可变 tag，例如 `ghcr.io/qwe819102926-dot/sub2api:sha-abcdef0`。

首次迁移到蓝绿部署前，确认 Caddy 配置和 Compose 文件位于同一台服务器，
并把仓库中的 `deploy/blue-green-update.sh` 复制到部署目录。

## 发布

```bash
cd /opt/sub2api-deploy
chmod +x blue-green-update.sh

./blue-green-update.sh \
  --image ghcr.io/qwe819102926-dot/sub2api:sha-abcdef0 \
  --public-health-url https://aitokey.top/health \
  --caddy-config /etc/caddy/Caddyfile
```

脚本会：

1. 拉取目标镜像并复制当前容器的环境变量、网络和所有挂载。
2. 在临时 loopback 端口 `18080` 启动 green。
3. 等待 green `/health` 成功。
4. 校验并 reload Caddy，将流量切换到 green。
5. 检查公网 `/health`。
6. 更新 `.env` 中的 `SUB2API_IMAGE`。
7. 默认等待 120 秒后停止旧容器。

第一次切换建议使用 `--keep-old`，验证一段时间后再停止旧容器。等待时间可用
`--drain-seconds 300` 调大，长连接较多时建议这样做。

## 回滚

如果切换后的公网检查失败，脚本会自动恢复 Caddy、`.env` 和旧容器。

手动回滚时，先找到本次部署目录里的旧 active 状态：

```bash
cat /opt/sub2api-deploy/.blue-green-active
cat /opt/sub2api-deploy/.blue-green/<timestamp>/active-state.backup
docker start <旧容器名>
```

然后把 Caddyfile 恢复为部署目录中对应运行目录的 `caddy.backup`，校验并 reload：

```bash
caddy validate --config /etc/caddy/Caddyfile --adapter caddyfile
caddy reload --config /etc/caddy/Caddyfile --adapter caddyfile
```

## 后台任务和数据安全

应用已经为多个周期任务接入 Redis leader lock，并在 Redis 不可用时回退到
PostgreSQL advisory lock。蓝绿部署期间两个应用实例共享 PostgreSQL、Redis
和 `/app/data`，因此不要启动第二套依赖服务或删除现有数据卷。

`SERVER_SHUTDOWN_TIMEOUT` 默认是 `120s`，控制 blue 收到 SIGTERM 后等待在途请求
结束的最长时间。它已写入生产 Compose 和环境变量模板。
