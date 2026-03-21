FROM golang:1.26-alpine AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN go build -o habitual ./cmd/habitual

FROM alpine:3.21

WORKDIR /app
COPY --from=builder /app/habitual .
COPY web/static ./web/static

EXPOSE 8080
CMD ["./habitual"]
