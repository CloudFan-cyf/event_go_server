# 构建阶段
FROM golang:1.21-alpine AS builder

# 设置工作目录
WORKDIR /app

# 复制依赖文件
COPY go.mod go.sum ./
RUN go mod download

# 复制源码
COPY . .

# 构建可执行文件
RUN CGO_ENABLED=0 GOOS=linux go build -o robot-server

# 最终运行阶段
FROM alpine:3.18

# 安装CA证书（用于HTTPS请求）
RUN apk --no-cache add ca-certificates

# 设置工作目录
WORKDIR /app

# 从构建阶段复制可执行文件
COPY --from=builder /app/robot-server .

# 暴露端口
EXPOSE 80

# 启动命令
CMD ["./robot-server"]
