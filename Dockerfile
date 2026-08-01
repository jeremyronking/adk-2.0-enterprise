# Multi-stage production build for ADK 2.0 Enterprise Microservices
FROM golang:1.26-alpine AS builder

ARG SERVICE=router

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o /app/bin/service ./cmd/${SERVICE}

FROM alpine:latest

RUN apk --no-cache add ca-certificates tzdata

WORKDIR /app

COPY --from=builder /app/bin/service /app/service

ENV PORT=8080
EXPOSE 8080

ENTRYPOINT ["/app/service"]
CMD ["web", "-port", "8080", "api", "webui"]
