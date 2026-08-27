# Sub2API 发布前测试方案(方案一 + 方案二)

本文档说明如何在本项目现有的「推 main → GitHub Actions 构建 ghcr.io 镜像 → 服务器手动更新」流程上,增加一道**先测试、后上线**的闸门。

```
改代码 → 推到 dev/任意分支
   │
   ▼
GitHub Actions: 构建镜像 + 冒烟测试(方案一)
   │ 通过
   ▼
推送 ghcr.io/.../sub2api:dev 镜像
   │
   ▼
服务器 staging 实例(方案二,与生产完全隔离)
   │ 你验证通过
   ▼
合并 dev → main
   │
   ▼
GitHub Actions: 冒烟测试通过后 才发布 latest(方案一闸门)
   │
   ▼
服务器手动 docker compose pull && up -d 更新生产(保持不变)
```

---

## 方案一:CI 冒烟测试(自动,不需要你操作)

新增/修改的两个 workflow:

### `.github/workflows/smoke-test.yml`(新)
- 触发:推送 `main` 之外的任意分支(即你的日常修改分支)
- 内容:
  1. 构建镜像(带 BuildKit 缓存,速度快)
  2. 在 GitHub runner 里临时启动 Postgres + Redis + Sub2API 容器
  3. 依次检查:`/health`、`/setup/status`、前端首页、管理员登录、带 token 的认证接口
  4. **全部通过**才推送 `ghcr.io/qwe819102926-dot/sub2api:dev` 和 `dev-<sha>` 镜像
- 冒烟逻辑在 `deploy/smoke-test.sh`,CI 和服务器都可以复用

### `.github/workflows/publish-image.yml`(修改)
- 在发布 `latest` 之前新增了 `smoke` 前置任务(`build` 任务 `needs: smoke`)
- 效果:**main 分支推送后,如果冒烟测试失败,`latest` 不会发布**,生产就不会拉到坏镜像
- 生产更新仍然保持你现在的「服务器手动 pull + up」,没有变化

### 手动触发冒烟
在 GitHub 仓库的 **Actions → Smoke test and dev image → Run workflow**,可以直接对任意分支/提交跑一次。

---

## 方案二:同服务器并行 staging 实例(零额外成本)

在**同一台生产服务器**上,用一套完全隔离的容器当测试环境,不碰生产的数据、端口、容器。

### 涉及文件
| 文件 | 作用 |
| --- | --- |
| `deploy/docker-compose.staging.yml` | staging 专用 compose(独立项目名/端口/卷/库名) |
| `deploy/.env.staging.example` | staging 环境变量模板(复制为 `.env.staging`) |

隔离点:
- 项目名 `sub2api-staging`(网络/卷全部独立)
- 容器名 `sub2api-staging` / `-postgres` / `-redis`
- 端口 `127.0.0.1:8081`(只走反向代理,不直接暴露公网)
- 卷 `sub2api_staging_data` / `postgres_staging_data` / `redis_staging_data`
- 数据库 `sub2api_staging`(和生产库完全分开)

### 服务器上一次性初始化(SSH 登录服务器执行)

```bash
# 1. 把这两个文件放到服务器(从本地 git 仓库拿,或直接编辑服务器副本)
#    /opt/sub2api-deploy/docker-compose.staging.yml
#    /opt/sub2api-deploy/.env.staging.example

cd /opt/sub2api-deploy

# 2. 生成 staging 环境变量(填 POSTGRES_PASSWORD / ADMIN_* / JWT_SECRET)
cp .env.staging.example .env.staging
chmod 600 .env.staging
nano .env.staging
#   必须填:
#     POSTGRES_PASSWORD=   (随便一个强密码,和生产的无关)
#     ADMIN_PASSWORD=      (staging 管理员的密码)
#     JWT_SECRET=          (openssl rand -hex 32)

# 3. 启动 staging(会拉取 :dev 镜像并初始化独立数据库)
docker compose \
  -f docker-compose.staging.yml \
  --env-file .env.staging \
  up -d

# 4. 看状态
docker compose -f docker-compose.staging.yml --env-file .env.staging ps
docker compose -f docker-compose.staging.yml --env-file .env.staging logs -f sub2api
```

### 反向代理(Caddy)给 staging 加个子域名

在你服务器现有的 Caddyfile 里加一段(Caddy 会自动签证书):

```caddyfile
staging.aitokey.top {
    tls {
        protocols tls1.2 tls1.3
    }

    reverse_proxy 127.0.0.1:8081 {
        health_uri /health
        health_interval 30s
        health_timeout 10s
        health_status 200

        header_up X-Real-IP {remote_host}
        header_up X-Forwarded-For {remote_host}
        header_up X-Forwarded-Proto {scheme}
        header_up X-Forwarded-Host {host}
    }

    encode {
        zstd
        gzip 6
        minimum_length 256
    }

    log {
        output file /var/log/caddy/sub2api-staging.log {
            roll_size 50mb
            roll_keep 10
            roll_keep_for 720h
        }
        format json
        level INFO
    }
}
```

改完后 `sudo systemctl reload caddy`(或 `caddy reload --config /path/Caddyfile`)。
然后浏览器访问 `https://staging.aitokey.top` 用 `.env.staging` 里的管理员账号登录测试。

> 注意:先在 DNS 服务商给 `staging.aitokey.top` 加一条 A 记录指向服务器 IP,否则 Caddy 无法签证书。

### 每次要测新代码时(日常流程)

```bash
# 本地
git push origin dev          # 或你随便开的分支

# GitHub Actions 会自动: 构建 + 冒烟测试 → 推送 :dev 镜像

# 服务器(更新 staging,只拉镜像,不动生产)
cd /opt/sub2api-deploy
docker compose -f docker-compose.staging.yml --env-file .env.staging pull sub2api
docker compose -f docker-compose.staging.yml --env-file .env.staging up -d
```

在 `https://staging.aitokey.top` 验证通过后,把分支合并进 `main` 走正常生产发布即可。

### 停止 / 清理 staging(不要的时候)

```bash
# 停止并删除容器(数据卷保留)
docker compose -f docker-compose.staging.yml --env-file .env.staging down

# 连数据卷一起删(慎重,staging 数据会丢)
docker compose -f docker-compose.staging.yml --env-file .env.staging down -v
```

---

## 回滚

- 生产镜像始终按 `sha-<commit>` 打 tag,出问题可以切回旧 tag(保持你现有的回滚做法)。
- staging 镜像也有 `dev-<sha>` tag,可固定到某个版本:
  ```bash
  # 把 staging 固定到某次提交的镜像(示例)
  sed -i 's#sub2api:dev#sub2api:dev-abc1234#' docker-compose.staging.yml
  docker compose -f docker-compose.staging.yml --env-file .env.staging up -d
  ```

---

## 需要留意的点

1. **镜像保留策略**:dev 镜像会随每次分支推送增长(dev + dev-<sha>)。可在
   GitHub → Packages → sub2api → Package settings 里开启自动清理/保留策略,只保留最近 N 个。
2. **staging 与生产共用服务器资源**:staging 默认只占 8081 端口 + 少量内存。
   如果服务器很紧张,可以只在要测试时启动 staging,测完 `down`。
3. **staging 不会自动更新**:每次需要手动 `pull + up`(脚本未做自动化,避免误操作)。
   如果之后想要「push 即自动部署到 staging」,可以再加一个带 SSH 密钥的 workflow,
   但建议先用人工拉取,更安全。
4. **假设**:staging compose 基于仓库 `deploy/docker-compose.yml` 生成,服务名
   `sub2api / postgres / redis` 与生产一致;如果服务器上的生产 compose 结构不同,
   按相同思路改成你自己的版本即可。