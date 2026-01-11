# 1. 构建阶段
FROM golang:1.22-alpine AS builder
WORKDIR /app

# 复制依赖文件并下载
COPY go.mod ./
# 如果你有 go.sum 也要复制，没有就算了
# COPY go.sum ./
RUN go mod download

# 复制源码并编译
COPY *.go ./
RUN go build -o server main.go

# 2. 运行阶段
FROM alpine:latest
WORKDIR /root/

# 安装时区数据 (确保日志和过期时间是北京时间)
RUN apk --no-cache add tzdata
ENV TZ=Asia/Shanghai

# 从构建阶段复制二进制文件
COPY --from=builder /app/server .

# 暴露端口
EXPOSE 8080

# 启动
CMD ["./server"]