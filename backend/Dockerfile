# Build stage
FROM golang:1.25-alpine AS builder

WORKDIR /app

RUN apk add --no-cache git ca-certificates

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o main ./server.go && \
    CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o migrate ./cmd/migrate && \
    CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o queuesweep ./cmd/queuesweep && \
    CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o sessionsweep ./cmd/sessionsweep && \
    CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o roomsweep ./cmd/roomsweep

# Final stage
FROM alpine:latest

RUN apk --no-cache add ca-certificates

RUN addgroup -g 1001 -S appgroup && \
    adduser -u 1001 -S appuser -G appgroup

WORKDIR /app

COPY --from=builder /app/main .
COPY --from=builder /app/migrate .
COPY --from=builder /app/queuesweep .
COPY --from=builder /app/sessionsweep .
COPY --from=builder /app/roomsweep .
COPY --from=builder /app/migrations ./migrations

RUN chown -R appuser:appgroup /app

USER appuser

EXPOSE 8080

HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
  CMD wget --no-verbose --tries=1 --spider http://localhost:8080/healthz || exit 1

CMD ["./main"]
