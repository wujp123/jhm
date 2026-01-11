# 构建阶段
FROM golang:1.22-alpine AS builder
WORKDIR /app
COPY . .
RUN go build -o server main.go

# 运行阶段
FROM alpine:latest
WORKDIR /root/
# 安装 tzdata 以支持 Asia/Shanghai 时区
RUN apk --no-cache add tzdata
COPY --from=builder /app/server .

# 暴露端口
EXPOSE 8080

# 启动命令
CMD ["./server"]