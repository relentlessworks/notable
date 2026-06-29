FROM golang:1.25-alpine AS builder

WORKDIR /app
COPY go.mod go.sum* ./
RUN go mod download 2>/dev/null || true
COPY . .
RUN CGO_ENABLED=0 go build -o notable ./cmd/notable

FROM alpine:3.20
RUN apk add --no-cache ca-certificates
WORKDIR /app
COPY --from=builder /app/notable .
EXPOSE 8080
VOLUME ["/app/data"]
ENV NOTABLE_DB=/app/data/notable.json
CMD ["./notable"]
