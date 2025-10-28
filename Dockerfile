# 多阶段构建 Dockerfile for PicaComic Server

# 构建阶段
FROM golang:1.21-alpine AS builder

WORKDIR /app

# 安装构建依赖
RUN apk add --no-cache gcc musl-dev sqlite-dev

# 复制 go.mod 和 go.sum
COPY go.mod go.sum ./

# 下载依赖
RUN go mod download

# 复制源代码
COPY . .

# 构建应用
# 添加 SQLite 编译标签和环境变量
RUN CGO_ENABLED=1 GOOS=linux \
    CGO_CFLAGS="-D_LARGEFILE64_SOURCE" \
    go build -a -installsuffix cgo -ldflags="-s -w" -tags sqlite_omit_load_extension -o pica-server main.go

# 运行阶段
FROM alpine:latest

WORKDIR /app

# 安装运行时依赖
RUN apk --no-cache add ca-certificates sqlite-libs

# 从构建阶段复制二进制文件
COPY --from=builder /app/pica-server .

# 创建数据目录
RUN mkdir -p /app/data /app/cache

# 暴露端口
EXPOSE 8080

# 设置环境变量
ENV GIN_MODE=release

# 健康检查
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
  CMD wget --no-verbose --tries=1 --spider http://localhost:8080/health || exit 1

# 运行应用
CMD ["./pica-server", "-host", "0.0.0.0", "-port", "8080"]

