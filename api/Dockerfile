FROM golang:1.26-alpine@sha256:f85330846cde1e57ca9ec309382da3b8e6ae3ab943d2739500e08c86393a21b1 AS builder
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

FROM alpine:3.23@sha256:5b10f432ef3da1b8d4c7eb6c487f2f5a8f096bc91145e68878dd4a5019afde11
RUN apk add --no-cache ca-certificates
COPY --from=builder /pypx-api /pypx-api
EXPOSE 8080
CMD ["/pypx-api"]
