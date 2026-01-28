#!/bin/bash
set -e

echo "=== Twitter Clone Deployment Script ==="

# Step 1: Create k3d cluster
echo ""
echo "Step 1: Creating k3d cluster..."
k3d cluster create twitter-k8s \
  --servers 1 \
  --agents 3 \
  --port "80:80@loadbalancer" \
  --port "443:443@loadbalancer" \
  --k3s-arg "--disable=traefik@server:0" || true

# Verify cluster
echo "Verifying cluster..."
kubectl get nodes

# Step 2: Install operators
echo ""
echo "Step 2: Installing operators..."

echo "Installing CloudNativePG operator..."
kubectl apply --server-side -f https://raw.githubusercontent.com/cloudnative-pg/cloudnative-pg/release-1.22/releases/cnpg-1.22.0.yaml

echo "Installing Redis operator..."
kubectl apply -f https://raw.githubusercontent.com/spotahome/redis-operator/master/manifests/databases.spotahome.com_redisfailovers.yaml
kubectl apply -f https://raw.githubusercontent.com/spotahome/redis-operator/master/example/operator/all-redis-operator-resources.yaml

echo "Installing ECK (Elasticsearch) operator..."
kubectl create -f https://download.elastic.co/downloads/eck/2.11.1/crds.yaml || true
kubectl apply -f https://download.elastic.co/downloads/eck/2.11.1/operator.yaml

echo "Installing RabbitMQ operator..."
kubectl apply -f https://github.com/rabbitmq/cluster-operator/releases/latest/download/cluster-operator.yml

echo "Installing Gateway API CRDs..."
kubectl apply -f https://github.com/kubernetes-sigs/gateway-api/releases/download/v1.0.0/standard-install.yaml

echo "Installing Envoy Gateway..."
kubectl apply --server-side -f https://github.com/envoyproxy/gateway/releases/download/v1.0.0/install.yaml

# Step 3: Wait for operators
echo ""
echo "Step 3: Waiting for operators to be ready..."
echo "Waiting for CloudNativePG..."
kubectl wait --for=condition=available deployment -l app.kubernetes.io/name=cloudnative-pg -n cnpg-system --timeout=300s || echo "CloudNativePG not ready yet"

echo "Waiting for Elastic operator..."
kubectl wait --for=condition=available statefulset -l control-plane=elastic-operator -n elastic-system --timeout=300s || echo "Elastic operator not ready yet"

echo "Waiting for RabbitMQ operator..."
kubectl wait --for=condition=available deployment -l app.kubernetes.io/name=rabbitmq-cluster-operator -n rabbitmq-system --timeout=300s || echo "RabbitMQ operator not ready yet"

echo "Waiting for Envoy Gateway..."
kubectl wait --for=condition=available deployment -l app.kubernetes.io/name=envoy-gateway -n envoy-gateway-system --timeout=300s || echo "Envoy Gateway not ready yet"

sleep 10

# Step 4: Deploy application
echo ""
echo "Step 4: Deploying Twitter Clone application..."
cd "$(dirname "$0")/.."
helm upgrade --install twitter ./helm/twitter-stack -n twitter --create-namespace

echo ""
echo "Step 5: Waiting for pods to be ready..."
sleep 5
kubectl get pods -n twitter

echo ""
echo "=== Deployment Complete ==="
echo "Run 'kubectl get pods -n twitter -w' to watch pod status"
echo "Run 'kubectl get svc -n twitter' to see services"
