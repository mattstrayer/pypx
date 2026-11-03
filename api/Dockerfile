FROM golang:1.27-alpine@sha256:d200d2dd76b03a99b52651fd1325aff7dcd35422c35c01bac56571b0507f3deb AS builder
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

FROM alpine:3.24@sha256:be2656061b536c1883c4034790e21d0a09c5aaaf51968b89368a3c0371ea2c9a
RUN apk add --no-cache ca-certificates
COPY --from=builder /pypx-api /pypx-api
EXPOSE 8080
CMD ["/pypx-api"]
