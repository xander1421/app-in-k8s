#!/bin/bash
set -e

echo "Installing Kubernetes Operators for Twitter Clone..."

# CloudNativePG (PostgreSQL)
echo "Installing CloudNativePG operator..."
kubectl apply -f https://raw.githubusercontent.com/cloudnative-pg/cloudnative-pg/release-1.22/releases/cnpg-1.22.0.yaml

# Wait for CloudNativePG to be ready
echo "Waiting for CloudNativePG operator to be ready..."
kubectl wait --for=condition=Available deployment/cnpg-controller-manager -n cnpg-system --timeout=120s

# Spotahome Redis Operator
echo "Installing Spotahome Redis operator..."
kubectl create namespace redis-operator --dry-run=client -o yaml | kubectl apply -f -
kubectl apply -f https://raw.githubusercontent.com/spotahome/redis-operator/master/manifests/databases.spotahome.com_redisfailovers.yaml
kubectl apply -f https://raw.githubusercontent.com/spotahome/redis-operator/master/example/operator/all-redis-operator-resources.yaml

# Wait for Redis operator
echo "Waiting for Redis operator to be ready..."
kubectl wait --for=condition=Available deployment/redisoperator -n redis-operator --timeout=120s || true

# ECK (Elasticsearch)
echo "Installing ECK operator..."
kubectl create -f https://download.elastic.co/downloads/eck/2.11.1/crds.yaml
kubectl apply -f https://download.elastic.co/downloads/eck/2.11.1/operator.yaml

# Wait for ECK operator
echo "Waiting for ECK operator to be ready..."
kubectl wait --for=condition=Available deployment/elastic-operator -n elastic-system --timeout=120s

# RabbitMQ Operator
echo "Installing RabbitMQ operator..."
kubectl apply -f https://github.com/rabbitmq/cluster-operator/releases/latest/download/cluster-operator.yml

# Wait for RabbitMQ operator
echo "Waiting for RabbitMQ operator to be ready..."
kubectl wait --for=condition=Available deployment/rabbitmq-cluster-operator -n rabbitmq-system --timeout=120s

# MinIO Operator
echo "Installing MinIO operator..."
kubectl apply -k "github.com/minio/operator?ref=v5.0.11"

# Wait for MinIO operator
echo "Waiting for MinIO operator to be ready..."
kubectl wait --for=condition=Available deployment/minio-operator -n minio-operator --timeout=120s || true

# Envoy Gateway
echo "Installing Envoy Gateway..."
helm install eg oci://docker.io/envoyproxy/gateway-helm --version v1.0.0 -n envoy-gateway-system --create-namespace

# Wait for Envoy Gateway
echo "Waiting for Envoy Gateway to be ready..."
kubectl wait --for=condition=Available deployment/envoy-gateway -n envoy-gateway-system --timeout=120s || true

# Create GatewayClass
echo "Creating GatewayClass..."
kubectl apply -f - <<EOF
apiVersion: gateway.networking.k8s.io/v1
kind: GatewayClass
metadata:
  name: envoy
spec:
  controllerName: gateway.envoyproxy.io/gatewayclass-controller
EOF

echo ""
echo "All operators installed successfully!"
echo ""
echo "Installed operators:"
echo "  - CloudNativePG (PostgreSQL)"
echo "  - Spotahome Redis Operator"
echo "  - ECK (Elasticsearch)"
echo "  - RabbitMQ Cluster Operator"
echo "  - MinIO Operator"
echo "  - Envoy Gateway"
echo ""
echo "You can now deploy the Twitter stack with:"
echo "  helm install twitter ./helm/twitter-stack -n twitter --create-namespace"
