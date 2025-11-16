FROM golang:1.23 AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o pr-reviewer-service ./cmd

FROM alpine:3.20

WORKDIR /app
COPY --from=builder /app/pr-reviewer-service /app/pr-reviewer-service

ENV HTTP_PORT=8080
ENV DB_DSN=postgres://pr_user:pr_pass@db:5432/pr_service?sslmode=disable

EXPOSE 8080

CMD ["/app/pr-reviewer-service"]