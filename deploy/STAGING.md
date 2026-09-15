# Sub2API 发布前测试方案(方案一 + 方案二)

本文档说明如何用 GitHub 托管 Runner **只构建一次**,先测试镜像 digest,再把同一
digest 提升为生产 tag。生产发布不重新编译镜像。

```
改代码 → 推到 dev
   │
   ▼
GitHub Actions: 构建 candidate-<完整 SHA> + 按 digest 冒烟(方案一)
   │ 通过
   ▼
保存 tested digest artifact,提升 :dev / :dev-<短 SHA>
   │
   ▼
服务器 staging 实例(方案二,与生产完全隔离)
   │ 你验证通过
   ▼
fast-forward dev → main(SHA 必须相同)
   │
   ▼
GitHub Actions: 校验 dev 成功记录和 digest artifact
   │
   ▼
远程提升同一 digest 为 :sha-<短 SHA> / :latest,再执行蓝绿发布
```

---

## 方案一:CI 冒烟测试(自动,不需要你操作)

新增/修改的两个 workflow:

### `.github/workflows/smoke-test.yml`
- 触发:推送 `dev` 或手动触发
- Runner:`ubuntu-latest`,不依赖自托管构建机
- 内容:
  1. 使用 BuildKit + GitHub Actions cache 构建并推送 `candidate-<完整 SHA>`
  2. 在 GitHub runner 里临时启动 Postgres + Redis + Sub2API 容器
  3. 依次检查:`/health`、`/setup/status`、前端首页、管理员登录、带 token 的认证接口
  4. 冒烟测试直接使用构建输出的 digest,并上传 `tested-image-<完整 SHA>` artifact
  5. **全部通过**才把同一 digest 提升为 `:dev` 和 `:dev-<短 SHA>`
- 冒烟逻辑在 `deploy/smoke-test.sh`,CI 和服务器都可以复用

### `.github/workflows/publish-image.yml`
- 不构建镜像,只接受在 dev 上成功测试过的同一个提交 SHA
- 下载对应 dev run 保存的 digest artifact,按 digest 提升为 `:sha-<短 SHA>` 和 `:latest`
- 在 main 上新产生、且未在 dev 测试过的 merge/rebase/direct commit SHA 会保护性失败
- 发布 dev 到 main 前,先把 main 的变化同步进 dev,再使用 fast-forward 保持 SHA 不变

### 手动触发冒烟
在 GitHub 仓库的 **Actions → Build and smoke-test dev image → Run workflow** 可以手动执行。
生产提升只认可 `dev` 分支 `push` 事件成功的结果,手动运行不能绕过发布闸门。

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
git push origin dev

# GitHub Actions 会自动: 构建 + 冒烟测试 → 推送 :dev 镜像

# 服务器(更新 staging,只拉镜像,不动生产)
cd /opt/sub2api-deploy
docker compose -f docker-compose.staging.yml --env-file .env.staging pull sub2api
docker compose -f docker-compose.staging.yml --env-file .env.staging up -d
```

在 `https://staging.aitokey.top` 验证通过后,先确认 dev 已包含 main 的全部提交,再将
main fast-forward 到该 dev SHA。可以先在 dev 合入 main 并测试该 merge commit,但不要在
main 上临时创建一个未经 dev 测试的新 merge commit,否则生产提升会拒绝执行。

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

1. **镜像保留策略**:dev 镜像会随每次推送增长(candidate-<完整 SHA> + dev-<短 SHA>)。可在
   GitHub → Packages → sub2api → Package settings 里开启自动清理/保留策略,只保留最近 N 个。
2. **staging 与生产共用服务器资源**:staging 默认只占 8081 端口 + 少量内存。
   如果服务器很紧张,可以只在要测试时启动 staging,测完 `down`。
3. **staging 不会自动更新**:每次需要手动 `pull + up`(脚本未做自动化,避免误操作)。
   如果之后想要「push 即自动部署到 staging」,可以再加一个带 SSH 密钥的 workflow,
   但建议先用人工拉取,更安全。
4. **假设**:staging compose 基于仓库 `deploy/docker-compose.yml` 生成,服务名
   `sub2api / postgres / redis` 与生产一致;如果服务器上的生产 compose 结构不同,
   按相同思路改成你自己的版本即可。
---

## 生产 / 测试隔离边界(保证)

以下为服务器实测确认的硬隔离点,生产与 staging 不会混用:

| 维度 | 生产 | staging(测试) |
|---|---|---|
| 数据库名 | `sub2api` | `sub2api_staging` |
| 数据库容器 | `sub2api-postgres` | `sub2api-staging-postgres` |
| 存储 | 宿主目录 bind-mount(`postgres_data/` `redis_data/` `data/`) | 独立命名卷 `*_staging_data` |
| 端口 | `127.0.0.1:8080` | `127.0.0.1:8081` |
| 镜像 tag | `ghcr.io/qwe819102926-dot/sub2api:latest` | `:dev` / `:dev-<sha>` |
| compose / env | `docker-compose.yml` + `.env` | `docker-compose.staging.yml` + `.env.staging` |
| 域名 | `aitokey.top` | `staging.aitokey.top` |
| 账号 | 生产管理员 | `admin-staging@aitokey.top` |

### 发布闸门(必须先测后发)

1. 新功能推 **dev** → CI 构建候选镜像 → 按 digest 冒烟 → 提升 `:dev` 镜像 → staging 更新。
2. 在 `https://staging.aitokey.top` 登录验证,确认无误。
3. 验证通过后,将 dev **fast-forward 到 main** → 校验相同 SHA 的成功 dev run → 将同一 digest 提升为生产 tag。
4. 使用 `sha-<短 SHA>` 不可变镜像执行蓝绿发布,生产数据全程使用现有 `sub2api` 数据库和 Redis。

### 防呆(避免误操作)

- 更新 **生产** 永远用默认 `docker compose`(即 `docker-compose.yml`);更新 **staging** 永远显式带 `-f docker-compose.staging.yml --env-file .env.staging`,不要靠默认。
- 不要改 staging 的 `POSTGRES_DB` 去指向 `sub2api`;`docker-compose.staging.yml` 默认就是 `sub2api_staging`,无需改动。
