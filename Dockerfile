FROM golang:1.27-alpine AS builder

WORKDIR /app

# Závislosti
COPY go.mod go.sum ./
RUN go mod download

# Zdrojové kódy
COPY . .

# Build statické binárky
# CGO_ENABLED=0 protože SQLite od modernc.org je pure-go, nepotřebujeme CGO!
RUN CGO_ENABLED=0 GOOS=linux go build -o /app/verifier-bot ./cmd/bot

FROM alpine:latest
RUN apk --no-cache add ca-certificates tzdata

WORKDIR /app
COPY --from=builder /app/verifier-bot .
COPY config.yml .

# Složka pro databázi
RUN mkdir -p /app/data

# Nespouštíme to pod rootem z bezpečnostních důvodů (volitelné, ale dobrá praxe)
# Zde si vystačíme s jednoduchým runem
CMD ["./verifier-bot"]
