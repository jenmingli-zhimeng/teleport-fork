# Teleport 集群 Docker 镜像

Auth + Proxy 单进程，`proxy_listener_mode: multiplex`，对外只暴露一个端口（默认 **27443**）。不要在前面再挂 HTTP 反向代理，否则 ALPN 多路复用和 Edge 反连隧道会坏掉。

本机（可访问 docker.io）负责构建完整镜像；目标机（中国、不能访问 docker.io）只 `docker load`。

## 目录

| 路径 | 用途 |
| --- | --- |
| `zhimeng/docker/profile/` | Dockerfile、compose、entrypoint、本说明 |
| `zhimeng/docker/images/` | `docker save` 出的镜像包 |
| `zhimeng/docker/tmp/` | 构建时的临时文件 |

## 1. 本机构建

```bash
cd zhimeng/docker/profile
./build.sh
```

产物：

- 镜像标签：`teleport-cluster:18.10.0`
- 文件：`zhimeng/docker/images/teleport-cluster-18.10.0-linux-amd64.tar.gz`

将 tar.gz、本目录下的 `docker-compose.yaml`、`.env.example`、空的 `certs/` 与 `logo/` 一起拷到目标机。

## 2. 目标机加载（离线）

```bash
docker load -i teleport-cluster-18.10.0-linux-amd64.tar.gz
cp .env.example .env
# 编辑 .env
mkdir -p data
docker compose up -d
```

不要 `docker compose pull`。

## 3. 可在容器外修改的项（`.env`）

| 变量 | 含义 |
| --- | --- |
| `TELEPORT_WEB_PORT` | 主机与容器监听端口，默认 `27443` |
| `TELEPORT_PUBLIC_HOST` / `TELEPORT_PUBLIC_ADDR` | 客户端和 Edge 看到的 `host:port` |
| `TELEPORT_AUTH_LISTEN_ADDR` | 容器内 Auth 地址，默认回环 `127.0.0.1:3025` |
| `TELEPORT_CERT_FILE` / `TELEPORT_KEY_FILE` | 主机上的证书与私钥路径 |
| `TELEPORT_HTTPS_RELOAD_INTERVAL` | 证书热加载间隔，默认 `1h`（配合 NPM/certbot 续期） |
| `TELEPORT_LOGO_LIGHT` / `TELEPORT_LOGO_DARK` | 主机上的 SVG Logo 路径；空文件则用镜像内置 Logo |
| `TELEPORT_DATA_DIR` | 集群数据目录 |
| `TELEPORT_CLUSTER_NAME` | 写入后端，事后不要改 |
| `TELEPORT_SECOND_FACTOR` | `otp`（默认）或 `off` |

证书示例（Let's Encrypt / Nginx Proxy Manager 落盘文件）：

```bash
TELEPORT_CERT_FILE=/etc/letsencrypt/live/teleport.example.com/fullchain.pem
TELEPORT_KEY_FILE=/etc/letsencrypt/live/teleport.example.com/privkey.pem
```

Logo 需为 SVG，Web UI 请求路径为 `/web/app/logo-light.svg` 与 `/web/app/logo-dark.svg`。

证书文件为空时，Teleport 使用自签证书（仅调试）。更换 `.env` 后执行 `docker compose up -d` 即可，不必重建镜像。

## 4. 首次使用

```bash
docker compose exec teleport tctl users add admin --roles=editor,access --logins=root
docker compose exec teleport tctl tokens add --type=node --ttl=24h
```

Edge 的 `proxy_server` 填 `TELEPORT_PUBLIC_HOST:TELEPORT_WEB_PORT`（例如 `teleport.example.com:27443`）。

## 5. 防火墙

只放行 `TELEPORT_WEB_PORT`（默认 27443/tcp）到公网。不要把 Auth 的 3025 暴露出去。
