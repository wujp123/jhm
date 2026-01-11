# 1. 构建阶段
FROM golang:1.22-alpine AS builder
WORKDIR /app

# 关键修改：禁用 CGO，确保生成静态二进制文件，防止兼容性问题
ENV CGO_ENABLED=0
ENV GOOS=linux

COPY go.mod ./
# 如果有 go.sum 就取消下面这行的注释
# COPY go.sum ./
RUN go mod download

COPY *.go ./
# 编译
RUN go build -o server main.go

# 2. 运行阶段
FROM alpine:latest
WORKDIR /root/

# 安装时区 (可选，如果不需要北京时间可去掉)
RUN apk --no-cache add tzdata
ENV TZ=Asia/Shanghai

COPY --from=builder /app/server .

# 明确暴露端口
EXPOSE 8080

# 启动
CMD ["./server"]