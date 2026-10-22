# RuleGo Server — Docker

## Quick start (Docker Compose)

```bash
curl -fsSL https://raw.githubusercontent.com/rulego/rulego/main/server/deploy/docker/docker-compose.yml -o docker-compose.yml
docker compose up -d
```

Open `http://<host>:9090/editor/` — default account `admin` / `admin`.
Change the password or set `RULEGO_REQUIRE_AUTH=true` before production use.

## Quick start (docker run)

```bash
docker run -d --name rulego-server -p 9090:9090 -v rulego-data:/app/data --restart unless-stopped rulego/rulego-server:latest
```

## Image variants / tags

| Tag | Meaning |
|---|---|
| `latest` | newest stable release |
| `1.0` / `1.0.0` | pinned minor / patch version |

All tags are multi-arch manifests (linux/amd64 + linux/arm64). The image ships the `with_all` build (all component packs: AI / IoT / ETL / CI / discovery …).

## Configuration

Default config ships in the image at `/app/config.conf`. Two ways to override, no file editing needed:

1. **Environment variables** — `RULEGO_<KEY>` where KEY is the upper-cased `config.conf` key:
   `RULEGO_SERVER=":9090"`, `RULEGO_DATA_DIR="/app/data"`, `RULEGO_REQUIRE_AUTH="true"`,
   `RULEGO_LOG_LEVEL="debug"`, `RULEGO_JWT_SECRET_KEY=...` (or plain `JWT_SECRET_KEY`, referenced by the shipped config).
2. **Mount your own config**: `-v ./config.conf:/app/config.conf:ro`.

All persistent state lives in `/app/data` (rule chains, versions, run logs, audit, IoT data, skills, system agents). Mount a volume there or the data is lost on container removal. `node_pool.json` and the embedded editor are read-only at runtime — no volume needed.

Kubernetes users: see `server/deploy/kubernetes/chart/rulego-server` (Helm, `oci://ghcr.io/rulego/charts/rulego-server`) or the generated `rulego-server-all-in-one.yaml`.

## Build from a release archive

```bash
tar -xzf rulego-server-all-linux-amd64.tar.gz -C rulego-image
docker build -t my/rulego-server rulego-image
```

The official image is assembled by CI (the editor frontend lives in a private repo, so the image cannot be built from source alone).

## 快速开始（中文）

```bash
curl -fsSL https://raw.githubusercontent.com/rulego/rulego/main/server/deploy/docker/docker-compose.yml -o docker-compose.yml
docker compose up -d
```

浏览器打开 `http://<主机IP>:9090/editor/`，默认账号 `admin`/`admin`（生产请改密或开 `RULEGO_REQUIRE_AUTH=true`）。镜像内置全组件 `with_all` 构建；常用配置用 `RULEGO_*` 环境变量覆盖或直接挂载自己的 `config.conf`；全部持久化状态在 `/app/data`，务必挂卷。K8s 用 Helm chart 或 all-in-one.yaml。
