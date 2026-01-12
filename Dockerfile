# 1. 构建阶段
FROM golang:1.22-alpine AS builder

WORKDIR /app

# 禁用 CGO 静态编译
ENV CGO_ENABLED=0
ENV GOOS=linux

# 复制依赖文件
COPY go.mod ./
# 如果有 go.sum 请取消下面这行的注释
# COPY go.sum ./

RUN go mod download

# 复制源码
COPY *.go ./

# 编译
RUN go build -o server main.go

# 2. 运行阶段
FROM alpine:latest

WORKDIR /root/

# 安装时区数据 (解决 time.LoadLocation 问题)
RUN apk --no-cache add tzdata
ENV TZ=Asia/Shanghai

# 从构建阶段复制二进制文件
COPY --from=builder /app/server .

ENV PORT=8080
# 🔥 关键修改：不要设置 ENV PORT=80，也不要 EXPOSE 80
# 改回 8080 只是作为一个默认提示，实际端口由 Deployra 决定
EXPOSE 8080

# 启动服务
CMD ["./server"]