# ---- 阶段 1：编译（golang 工具链 + 源码 → 一个静态二进制） ----
FROM golang:1.26-alpine AS builder

# 国内服务器拉模块加速；能直连 proxy.golang.org 就删掉这行
ENV GOPROXY=https://goproxy.cn,direct
# CGO 关闭 → 纯静态链接：运行镜像不需要编译器和 libc，体积从 ~800MB 缩到 ~20MB
# （本项目 argon2 / go-redis / mysql driver 全是纯 Go，没有 CGO 依赖）
ENV CGO_ENABLED=0

WORKDIR /src

# 先只拷依赖清单再 download：只要 go.mod/go.sum 没变，改代码重建时这层直接命中缓存
COPY go.mod go.sum ./
RUN go mod download

COPY . .
# -s -w 去掉符号表/调试信息；-trimpath 去掉本机绝对路径（二进制里不泄露构建机目录结构）
RUN go build -trimpath -ldflags="-s -w" -o /out/api ./cmd/api

# ---- 阶段 2：运行时（只要二进制 + 时区数据 + 非 root 用户） ----
FROM alpine:3.22

# tzdata：DSN 里 loc=Local + DATETIME 全用本地时区，容器时区必须与 MySQL 容器一致，
# 否则录入时间差 8 小时（这是容器化 MySQL 应用最常见的坑之一）
RUN apk add --no-cache tzdata ca-certificates \
    && adduser -D -u 10001 app \
    && mkdir -p /app/uploads && chown -R app:app /app

ENV TZ=Asia/Shanghai
# 非 root 运行：8080 是非特权端口，容器被攻破也拿不到 root
USER app
WORKDIR /app

COPY --from=builder /out/api /usr/local/bin/api

EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/api"]
