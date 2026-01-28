#!/bin/bash
set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(dirname "$SCRIPT_DIR")"

cd "$ROOT_DIR"

REGISTRY="${REGISTRY:-ghcr.io/alexprut/twitter-clone}"
TAG="${TAG:-latest}"

SERVICES=(
    "user-service"
    "tweet-service"
    "timeline-service"
    "fanout-service"
    "media-service"
    "notification-service"
    "search-service"
)

echo "Building Twitter Clone services..."
echo "Registry: $REGISTRY"
echo "Tag: $TAG"
echo ""

# Build Go binaries
echo "Building Go binaries..."
for service in "${SERVICES[@]}"; do
    echo "  Building $service..."

    if [ "$service" == "fanout-service" ]; then
        CMD_PATH="cmd/worker"
    else
        CMD_PATH="cmd/server"
    fi

    (cd "services/$service" && \
        CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
        go build -ldflags="-w -s" -o "../../bin/$service" "./$CMD_PATH")
done

echo ""
echo "Building Docker images..."
for service in "${SERVICES[@]}"; do
    echo "  Building image for $service..."

    if [ "$service" == "fanout-service" ]; then
        CMD_PATH="cmd/worker"
    else
        CMD_PATH="cmd/server"
    fi

    docker build -t "$REGISTRY/$service:$TAG" \
        --build-arg SERVICE="$service" \
        --build-arg CMD_PATH="$CMD_PATH" \
        -f Dockerfile.service \
        .
done

echo ""
echo "All services built successfully!"
echo ""
echo "To push images:"
echo "  docker push $REGISTRY/user-service:$TAG"
echo "  # ... repeat for all services"
echo ""
echo "Or use: ./scripts/push-all.sh"
