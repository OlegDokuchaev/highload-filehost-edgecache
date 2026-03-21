FROM golang:1.22-alpine AS builder

WORKDIR /app

# Copy module files first to cache dependencies.
COPY src/go.mod src/go.sum* ./
RUN go mod download

# Copy source code and build.
COPY src/ ./
RUN CGO_ENABLED=0 GOOS=linux go build -o origin-service .

FROM alpine:3.20

WORKDIR /app

COPY --from=builder /app/origin-service /usr/local/bin/origin-service

EXPOSE 8080

ENTRYPOINT ["/usr/local/bin/origin-service"]
