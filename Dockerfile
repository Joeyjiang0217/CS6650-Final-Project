FROM golang:1.25-alpine AS builder

WORKDIR /app

COPY mnt/data/chatserver_split/go.mod mnt/data/chatserver_split/go.sum ./
RUN go mod download

COPY mnt/data/chatserver_split/ .

RUN CGO_ENABLED=0 GOOS=linux go build -o server .

FROM alpine:3.20

WORKDIR /app
COPY --from=builder /app/server /app/server

EXPOSE 8080

CMD ["/app/server"]