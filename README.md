# FileShare - Go HTTP/3 File Sharing Application

A production-ready file sharing application built with Go, featuring HTTP/3 (QUIC) support, deployed on Kubernetes with Helm and operator-managed infrastructure.

## Architecture

```
                           ┌─────────────────┐
                           │  Envoy Gateway  │
                           │   (HTTP/HTTPS)  │
                           └────────┬────────┘
                                    │
                           ┌────────▼────────┐
                           │   Go FileShare  │
                           │   (3 replicas)  │
                           │  HTTP/2 + HTTP/3│
                           └────────┬────────┘
                                    │
        ┌───────────┬───────────┬───┴───┬───────────┐
        │           │           │       │           │
   ┌────▼────┐ ┌────▼────┐ ┌────▼───┐ ┌─▼──┐ ┌─────▼─────┐
   │PostgreSQL│ │  Redis  │ │RabbitMQ│ │ ES │ │  Shared   │
   │ (CNPG)  │ │Sentinel │ │Cluster │ │    │ │  Storage  │
   │ 3 nodes │ │  3+3    │ │3 nodes │ │ 3  │ │(emptyDir) │
   └─────────┘ └─────────┘ └────────┘ └────┘ └───────────┘
```

## Features

- **HTTP/3 (QUIC)** - Ultra-fast file transfers via quic-go
- **HTTP/2** - Multiplexed connections with TLS
- **File Operations** - Upload, download, list, delete, share
- **Full-text Search** - Elasticsearch-powered file search
- **Background Jobs** - RabbitMQ for async processing (thumbnails, notifications)
- **Caching & Pub/Sub** - Redis Sentinel for caching and cluster events
- **HA Database** - CloudNativePG PostgreSQL with 3 replicas
- **Zero-downtime Deployments** - Rolling updates with PDB

## API Endpoints

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/health` | Liveness probe |
| GET | `/health/ready` | Readiness probe (checks all services) |
| GET | `/cluster` | Cluster info (instance, peers, uptime) |
| POST | `/files` | Upload file |
| GET | `/files` | List all files |
| GET | `/files/{id}` | Get file metadata |
| GET | `/files/{id}/download` | Download file |
| DELETE | `/files/{id}` | Delete file |
| GET | `/search?q=` | Search files (Elasticsearch) |
| POST | `/files/{id}/share` | Create share link |
| GET | `/share/{token}` | Access shared file |
| GET | `/queues` | Queue statistics (RabbitMQ) |

## Prerequisites

- Docker
- kubectl
- Helm 3.x
- k3d (for local development)

## Quick Start

### 1. Create k3d Cluster

```bash
k3d cluster create app-k8s \
  --servers 1 \
  --agents 2 \
  --port "80:80@loadbalancer" \
  --port "443:443@loadbalancer"
```

### 2. Install Operators

```bash
./helm/goapp-stack/scripts/install-operators.sh
```

This installs:
- **CloudNativePG** - PostgreSQL operator
- **Spotahome Redis Operator** - Redis with Sentinel
- **ECK** - Elasticsearch operator
- **RabbitMQ Cluster Operator** - RabbitMQ operator
- **Envoy Gateway** - API Gateway

### 3. Build and Import Image

```bash
# Build the Go application
docker build -t fileshare:latest .

# Import into k3d
k3d image import fileshare:latest -c app-k8s
```

### 4. Deploy with Helm

```bash
# Create namespace with PSS labels
kubectl create namespace goapp
kubectl label namespace goapp \
  pod-security.kubernetes.io/enforce=baseline \
  pod-security.kubernetes.io/warn=baseline

# Deploy
helm install goapp ./helm/goapp-stack \
  -n goapp \
  --set goapp.image.repository=fileshare \
  --set goapp.image.tag=latest \
  --set goapp.image.pullPolicy=Never
```

### 5. Enable HTTP/3 (Optional)

```bash
helm upgrade goapp ./helm/goapp-stack \
  -n goapp \
  --set goapp.http3.enabled=true \
  --set goapp.image.pullPolicy=Never
```

## Configuration

### Helm Values

| Value | Default | Description |
|-------|---------|-------------|
| `goapp.replicas` | 3 | Number of app replicas |
| `goapp.port` | 8080 | Application port |
| `goapp.http3.enabled` | false | Enable HTTP/3 (QUIC) |
| `postgresql.enabled` | true | Deploy PostgreSQL cluster |
| `postgresql.instances` | 3 | PostgreSQL replicas |
| `redis.enabled` | true | Deploy Redis with Sentinel |
| `redis.replicas` | 3 | Redis replicas |
| `elasticsearch.enabled` | true | Deploy Elasticsearch |
| `elasticsearch.nodes` | 3 | Elasticsearch nodes |
| `rabbitmq.enabled` | true | Deploy RabbitMQ cluster |
| `rabbitmq.replicas` | 3 | RabbitMQ replicas |
| `networkPolicy.enabled` | true | Enable NetworkPolicies |
| `gateway.enabled` | true | Deploy Envoy Gateway |

### Environment Variables

| Variable | Description |
|----------|-------------|
| `DATABASE_URL` | PostgreSQL connection URI (from CNPG secret) |
| `REDIS_SENTINEL_ADDR` | Redis Sentinel address |
| `REDIS_MASTER_NAME` | Redis master name (default: mymaster) |
| `ELASTICSEARCH_URL` | Elasticsearch HTTP endpoint |
| `RABBITMQ_URL` | RabbitMQ connection string (from operator secret) |

## Security Features

- **Pod Security Standards** - Baseline enforcement at namespace level
- **SecurityContext** - Non-root user (65532), read-only filesystem, dropped capabilities
- **ServiceAccount** - Dedicated SA with no token automount
- **NetworkPolicies** - Zero-trust networking for all components
- **Seccomp** - RuntimeDefault profile (optional custom profiles)
- **AppArmor** - Runtime/default profile support

## Monitoring

### Check Component Status

```bash
# PostgreSQL
kubectl get clusters.postgresql.cnpg.io -n goapp

# Redis
kubectl get redisfailovers -n goapp

# Elasticsearch
kubectl get elasticsearch -n goapp

# RabbitMQ
kubectl get rabbitmqclusters -n goapp

# Application
kubectl get pods -n goapp -l app=fileshare
```

### Health Checks

```bash
# Port forward
kubectl port-forward svc/goapp-goapp 8080:8080 -n goapp

# Check health (HTTP)
curl http://localhost:8080/health

# Check readiness (all services)
curl http://localhost:8080/health/ready

# With HTTP/3 enabled (HTTPS)
curl -k https://localhost:8080/health
```

## Testing

### Run Helm Tests

```bash
helm unittest ./helm/goapp-stack
```

### Test File Operations

```bash
# Upload
curl -X POST -F "file=@test.txt" http://localhost:8080/files

# List
curl http://localhost:8080/files

# Download
curl http://localhost:8080/files/{id}/download

# Search
curl "http://localhost:8080/search?q=test"
```

## Project Structure

```
.
├── cmd/server/          # Application entrypoint
├── internal/
│   ├── cache/           # Redis client (Sentinel)
│   ├── config/          # Configuration loading
│   ├── database/        # PostgreSQL client
│   ├── handlers/        # HTTP handlers
│   ├── models/          # Data models
│   ├── queue/           # RabbitMQ client
│   ├── search/          # Elasticsearch client
│   └── server/          # HTTP/2 + HTTP/3 server
├── helm/goapp-stack/    # Helm chart
│   ├── templates/       # Kubernetes manifests
│   ├── tests/           # Helm unit tests
│   ├── scripts/         # Operator installation
│   └── values.yaml      # Default values
└── Dockerfile           # Multi-stage distroless build
```

## Operators Used

| Service | Operator | CRD |
|---------|----------|-----|
| PostgreSQL | CloudNativePG | `Cluster` |
| Redis | Spotahome Redis Operator | `RedisFailover` |
| Elasticsearch | ECK | `Elasticsearch` |
| RabbitMQ | RabbitMQ Cluster Operator | `RabbitmqCluster` |
| Gateway | Envoy Gateway | `Gateway`, `HTTPRoute` |

## Troubleshooting

### Pods not starting

```bash
# Check events
kubectl get events -n goapp --sort-by='.lastTimestamp'

# Check pod logs
kubectl logs -n goapp -l app=fileshare --tail=50
```

### Database connection issues

```bash
# Check PostgreSQL cluster status
kubectl get clusters.postgresql.cnpg.io goapp-postgresql -n goapp -o wide

# Get connection secret
kubectl get secret goapp-postgresql-app -n goapp -o jsonpath='{.data.uri}' | base64 -d
```

### Redis Sentinel issues

```bash
# Check RedisFailover status
kubectl get redisfailovers goapp-redis -n goapp

# Check Sentinel logs
kubectl logs -n goapp -l app.kubernetes.io/component=sentinel --tail=20
```

## License

MIT
