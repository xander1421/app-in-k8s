# Build stage
FROM golang:1.24-alpine AS builder

WORKDIR /app

# Copy the entire project context (needed for pkg dependencies)
COPY go.work go.work.sum ./
COPY pkg ./pkg
COPY services ./services

# Navigate to service directory and build
WORKDIR /app/services/timeline-service
RUN go mod download
RUN CGO_ENABLED=0 GOOS=linux go build -o main ./cmd/server/main.go

# Final stage
FROM alpine:latest

# Install ca-certificates for HTTPS calls
RUN apk --no-cache add ca-certificates

WORKDIR /root/

# Copy binary from builder stage
COPY --from=builder /app/services/timeline-service/main .

# Certificates not needed for this service

# Expose port
EXPOSE 8080

# Run the binary
CMD ["./main"]