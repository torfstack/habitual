FROM cgr.dev/chainguard/go:latest AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o habitual ./cmd/habitual

FROM cgr.dev/chainguard/static:latest

WORKDIR /app
COPY --from=builder /app/habitual .
COPY web/static ./web/static

EXPOSE 8080
CMD ["/app/habitual"]
