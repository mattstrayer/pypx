FROM golang:1.27-alpine@sha256:cf6fca6641884b8433441b2b0652976f975e1d0fdd26d177eaaf8596087f3125 AS builder
WORKDIR /app

# Copy goopy module first (local dependency).
COPY goopy/ ./goopy/

# Copy API module and download deps.
COPY api/go.mod api/go.sum ./api/
WORKDIR /app/api
RUN go mod download

# Copy API source and build.
COPY api/ ./
RUN CGO_ENABLED=0 go build -o /pypx-api ./cmd/server

FROM alpine:3.24@sha256:28bd5fe8b56d1bd048e5babf5b10710ebe0bae67db86916198a6eec434943f8b
RUN apk add --no-cache ca-certificates
COPY --from=builder /pypx-api /pypx-api
EXPOSE 8080
CMD ["/pypx-api"]
