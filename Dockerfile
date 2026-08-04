FROM golang:1.21-alpine AS builder

WORKDIR /app
COPY go-agent/go.mod go-agent/go.sum ./go-agent/
COPY go-agent/ ./go-agent/

WORKDIR /app/go-agent
RUN CGO_ENABLED=0 GOOS=linux go build -o /app/beryl7-agent ./cmd

FROM alpine:latest
RUN apk add --no-cache ca-certificates curl bash sqlite

WORKDIR /root
COPY --from=builder /app/beryl7-agent /usr/bin/beryl7-agent

EXPOSE 8888
CMD ["/usr/bin/beryl7-agent", "-config", "/etc/beryl7/agent.env"]
