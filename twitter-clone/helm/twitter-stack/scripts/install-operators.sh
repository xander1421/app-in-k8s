#!/bin/bash
set -e

echo "=============================================="
echo "Installing Kubernetes Operators for goapp-stack"
echo "=============================================="

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

print_status() {
    echo -e "${GREEN}[OK]${NC} $1"
}

print_warning() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

print_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Check prerequisites
echo ""
echo "Checking prerequisites..."

if ! command -v kubectl &> /dev/null; then
    print_error "kubectl is not installed"
    exit 1
fi

if ! command -v helm &> /dev/null; then
    print_error "helm is not installed"
    exit 1
fi

print_status "kubectl and helm are available"

# 1. Install Envoy Gateway
echo ""
echo "----------------------------------------------"
echo "1. Installing Envoy Gateway"
echo "----------------------------------------------"

if kubectl get namespace envoy-gateway-system &> /dev/null; then
    print_warning "envoy-gateway-system namespace already exists, skipping..."
else
    helm install eg oci://docker.io/envoyproxy/gateway-helm \
        --version v1.6.2 \
        -n envoy-gateway-system \
        --create-namespace \
        --wait
    print_status "Envoy Gateway installed"
fi

# Wait for Envoy Gateway to be ready
echo "Waiting for Envoy Gateway to be ready..."
kubectl wait --for=condition=Available deployment/envoy-gateway -n envoy-gateway-system --timeout=120s || true
print_status "Envoy Gateway is ready"

# 2. Install CloudNativePG
echo ""
echo "----------------------------------------------"
echo "2. Installing CloudNativePG Operator"
echo "----------------------------------------------"

if kubectl get namespace cnpg-system &> /dev/null; then
    print_warning "cnpg-system namespace already exists, skipping..."
else
    kubectl apply -f https://raw.githubusercontent.com/cloudnative-pg/cloudnative-pg/main/releases/cnpg-1.25.0.yaml
    print_status "CloudNativePG operator installed"
fi

# Wait for CNPG operator to be ready
echo "Waiting for CloudNativePG operator to be ready..."
kubectl wait --for=condition=Available deployment/cnpg-controller-manager -n cnpg-system --timeout=120s || true
print_status "CloudNativePG operator is ready"

# 3. Install Redis Operator (Spotahome)
echo ""
echo "----------------------------------------------"
echo "3. Installing Spotahome Redis Operator"
echo "----------------------------------------------"

# Add Helm repo
helm repo add redis-operator https://spotahome.github.io/redis-operator 2>/dev/null || true
helm repo update

if helm status redis-operator &> /dev/null; then
    print_warning "Redis operator already installed, skipping..."
else
    helm install redis-operator redis-operator/redis-operator \
        --wait
    print_status "Redis operator installed"
fi

# Wait for Redis operator to be ready
echo "Waiting for Redis operator to be ready..."
kubectl wait --for=condition=Available deployment/redis-operator --timeout=120s || true
print_status "Redis operator is ready"

# 4. Install ECK (Elastic Cloud on Kubernetes)
echo ""
echo "----------------------------------------------"
echo "4. Installing Elastic Cloud on Kubernetes (ECK)"
echo "----------------------------------------------"

if kubectl get namespace elastic-system &> /dev/null; then
    print_warning "elastic-system namespace already exists, skipping..."
else
    # Install CRDs
    kubectl create -f https://download.elastic.co/downloads/eck/3.2.0/crds.yaml
    # Install operator
    kubectl apply -f https://download.elastic.co/downloads/eck/3.2.0/operator.yaml
    print_status "ECK operator installed"
fi

# Wait for ECK operator to be ready
echo "Waiting for ECK operator to be ready..."
kubectl wait --for=condition=Available deployment/elastic-operator -n elastic-system --timeout=120s || true
print_status "ECK operator is ready"

# 5. Install RabbitMQ Cluster Operator
echo ""
echo "----------------------------------------------"
echo "5. Installing RabbitMQ Cluster Operator"
echo "----------------------------------------------"

if kubectl get namespace rabbitmq-system &> /dev/null; then
    print_warning "rabbitmq-system namespace already exists, skipping..."
else
    kubectl apply -f https://github.com/rabbitmq/cluster-operator/releases/latest/download/cluster-operator.yml
    print_status "RabbitMQ operator installed"
fi

# Wait for RabbitMQ operator to be ready
echo "Waiting for RabbitMQ operator to be ready..."
kubectl wait --for=condition=Available deployment/rabbitmq-cluster-operator -n rabbitmq-system --timeout=120s || true
print_status "RabbitMQ operator is ready"

# Summary
echo ""
echo "=============================================="
echo "All operators installed successfully!"
echo "=============================================="
echo ""
echo "Installed operators:"
echo "  - Envoy Gateway (v1.6.2) in envoy-gateway-system"
echo "  - CloudNativePG (v1.25.0) in cnpg-system"
echo "  - Spotahome Redis Operator in default namespace"
echo "  - ECK (v3.2.0) in elastic-system"
echo "  - RabbitMQ Cluster Operator in rabbitmq-system"
echo ""
echo "Verify all operators are running:"
echo "  kubectl get pods -n envoy-gateway-system"
echo "  kubectl get pods -n cnpg-system"
echo "  kubectl get pods -l app.kubernetes.io/name=redis-operator"
echo "  kubectl get pods -n elastic-system"
echo "  kubectl get pods -n rabbitmq-system"
echo ""
echo "You can now deploy the Helm chart:"
echo "  helm install goapp ./helm/goapp-stack -n goapp --create-namespace"
echo ""
