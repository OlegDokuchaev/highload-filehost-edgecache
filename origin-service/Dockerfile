FROM golang:1.22-alpine AS builder

WORKDIR /app

COPY src/go.mod src/go.sum* ./
RUN go mod download

COPY src/ ./
RUN go mod tidy && go build -o origin-service .

FROM alpine:3.20

WORKDIR /app

COPY --from=builder /app/origin-service /usr/local/bin/origin-service

EXPOSE 8080

ENTRYPOINT ["/usr/local/bin/origin-service"]
