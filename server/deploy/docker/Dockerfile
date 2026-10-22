# syntax=docker/dockerfile:1
# rulego-server 镜像：runtime 只打包官方 release 产物（静态二进制 + editor 前端 + 默认配置）。
# 构建上下文 = 解压后的 rulego-server-all-linux-<arch>.tar.gz（CI 组装，见 build.yml docker job）。
# 本地用户也可对任意 release 归档自行构建：
#   tar -xzf rulego-server-all-linux-amd64.tar.gz -C rulego-image && docker build -t rulego-server rulego-image
# editor 前端在私有仓，无法容器内从源码复现构建；binary 为 CGO_ENABLED=0 静态链接，alpine 即可运行。
FROM alpine:3.20
# ca-certificates：出站 HTTPS（LLM/Webhook/市场）；tzdata：定时调度按本地时区
RUN apk add --no-cache ca-certificates tzdata
WORKDIR /app
COPY . /app/
# release 归档里的二进制带平台后缀（rulego-server-all-linux-amd64 等），统一改名供 ENTRYPOINT 引用
RUN [ -f ./rulego-server ] || mv ./rulego-server-* ./rulego-server; chmod +x ./rulego-server
# data/ 是全部持久化状态（链/版本/运行日志/审计/IoT），必须挂卷
VOLUME ["/app/data"]
EXPOSE 9090
# 假定默认监听 :9090；用 RULEGO_SERVER 改端口的部署请自行覆盖 HEALTHCHECK 或改用编排层探针
HEALTHCHECK --interval=30s --timeout=5s --start-period=60s --retries=3 \
  CMD wget -qO- http://127.0.0.1:9090/api/v1/ping >/dev/null 2>&1 || exit 1
ENTRYPOINT ["./rulego-server"]
