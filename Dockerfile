FROM golang:1.22-alpine AS builder
WORKDIR /app
ENV CGO_ENABLED=0
ENV GOOS=linux
COPY go.mod ./
RUN go mod download
COPY *.go ./
RUN go build -o server main.go

FROM alpine:latest
WORKDIR /root/
RUN apk --no-cache add tzdata libcap libcap-utils
ENV TZ=Asia/Shanghai

COPY --from=builder /app/server .

RUN setcap 'cap_net_bind_service=+ep' /root/server

EXPOSE 80
EXPOSE 8080

CMD ["./server"]