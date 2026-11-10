FROM golang:1.27-alpine@sha256:eb61c3873c97416fff0116553fff65d67a3549f72e11d9758849a689cdd8b523 AS builder
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

FROM alpine:3.24@sha256:f59abe656b692d901e184be3445e084786602108599d2a51d229c0f5a48c71bf
RUN apk add --no-cache ca-certificates
COPY --from=builder /pypx-api /pypx-api
EXPOSE 8080
CMD ["/pypx-api"]
