FROM golang:1.27-alpine@sha256:4c9fe60190a2a3350ddc51de80d0224b8a6698d12bdfc999fee45ea9d6c46dbc AS builder
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
