# 1. 构建阶段
FROM golang:1.22-alpine AS builder

WORKDIR /app

# 禁用 CGO
ENV CGO_ENABLED=0
ENV GOOS=linux

COPY go.mod ./
# COPY go.sum ./
RUN go mod download

COPY *.go ./
RUN go build -o server main.go

# 2. 运行阶段
FROM alpine:latest

# 安装基础库和时区
RUN apk --no-cache add tzdata ca-certificates
ENV TZ=Asia/Shanghai

WORKDIR /app

# 从构建阶段复制
COPY --from=builder /app/server .

# 🔥 强制设置环境变量，防止外部干扰
ENV PORT=8080

# 暴露端口
EXPOSE 8080

# 启动命令
CMD ["./server"]