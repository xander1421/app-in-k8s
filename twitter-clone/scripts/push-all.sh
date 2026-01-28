#!/bin/bash
set -e

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

echo "Pushing Twitter Clone images to $REGISTRY..."
echo ""

for service in "${SERVICES[@]}"; do
    echo "Pushing $service..."
    docker push "$REGISTRY/$service:$TAG"
done

echo ""
echo "All images pushed successfully!"
