# 1. 构建阶段
FROM golang:1.22-alpine AS builder
WORKDIR /app
# 禁用 CGO 静态编译
ENV CGO_ENABLED=0
ENV GOOS=linux
COPY go.mod ./
RUN go mod download
COPY *.go ./
RUN go build -o server main.go

# 2. 运行阶段
FROM alpine:latest
WORKDIR /root/
RUN apk --no-cache add tzdata
ENV TZ=Asia/Shanghai
COPY --from=builder /app/server .

RUN setcap 'cap_net_bind_service=+ep' /root/server
# --- 修改这里：改为暴露 80 ---
EXPOSE 80

CMD ["./server"]