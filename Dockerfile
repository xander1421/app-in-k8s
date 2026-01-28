# Build stage
FROM golang:1.22-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git ca-certificates tzdata

WORKDIR /build

# Copy go mod files first for better caching
COPY go.mod go.sum* ./
RUN go mod download

# Copy source
COPY . .

# Build static binary
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags="-w -s -extldflags '-static'" \
    -o /fileshare \
    ./cmd/server

# Production stage - distroless for security
FROM gcr.io/distroless/static:nonroot

# Copy timezone data
COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo

# Copy CA certs for HTTPS
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

# Copy binary
COPY --from=builder /fileshare /fileshare

# Create uploads directory (will be mounted as volume)
# Note: distroless doesn't have mkdir, so we rely on the app or volume mount

# Use non-root user (65532 is nonroot in distroless)
USER 65532:65532

# Expose ports
# 8080 - HTTP/1.1 and HTTP/2
# 8443 - HTTP/3 (QUIC/UDP) when TLS enabled
EXPOSE 8080 8443/udp

# Health check
HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
    CMD ["/fileshare", "-health"] || exit 1

ENTRYPOINT ["/fileshare"]
