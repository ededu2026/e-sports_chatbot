FROM golang:1.22-alpine AS builder

WORKDIR /app

COPY go.mod ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /esports-chatbot ./cmd/server

FROM alpine:3.20

WORKDIR /app
RUN apk add --no-cache ca-certificates

COPY --from=builder /esports-chatbot /usr/local/bin/esports-chatbot
COPY --from=builder /app/knowledge_base /app/knowledge_base

EXPOSE 8080

CMD ["esports-chatbot"]
