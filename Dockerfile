# 1. 构建阶段
FROM golang:1.22-alpine AS builder
WORKDIR /app

ENV CGO_ENABLED=0
ENV GOOS=linux

COPY go.mod ./
RUN go mod download
COPY *.go ./
RUN go build -o server main.go


# 2. 运行阶段
FROM alpine:latest
WORKDIR /root/

# 🔥 关键：必须安装 libcap-utils 才有 setcap
RUN apk --no-cache add tzdata libcap libcap-utils

ENV TZ=Asia/Shanghai

COPY --from=builder /app/server .

# 🔥 允许非 root 进程绑定 80 端口
RUN setcap 'cap_net_bind_service=+ep' /root/server

EXPOSE 80

CMD ["./server"]