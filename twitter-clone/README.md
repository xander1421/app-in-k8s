# Twitter Clone - Microservices Architecture with Go

A production-ready Twitter clone built with Go microservices, featuring HTTP/3 support, deployed on Kubernetes with operator-managed infrastructure.

## 🏗️ Architecture Overview

```
                            ┌─────────────────┐
                            │  Envoy Gateway  │
                            │   HTTP/3 Ready  │
                            └────────┬────────┘
                                     │
        ┌────────────────────────────┼────────────────────────────┐
        │                            │                            │
   ┌────▼────┐  ┌─────▼─────┐  ┌────▼────┐  ┌────▼────┐  ┌──────▼──────┐
   │  User   │  │   Tweet   │  │Timeline │  │ Search  │  │   Media     │
   │ Service │  │  Service  │  │ Service │  │ Service │  │   Service   │
   │ HTTP/3  │  │  HTTP/3   │  │ HTTP/3  │  │ HTTP/3  │  │   HTTP/3    │
   └────┬────┘  └─────┬─────┘  └────┬────┘  └────┬────┘  └──────┬──────┘
        │             │              │            │               │
   ┌────▼────────────▼──────────────▼────────────▼───────────────▼────┐
   │                     Shared Infrastructure Layer                   │
   ├────────────────────────────────────────────────────────────────────┤
   │ PostgreSQL │ Redis Cluster │ Elasticsearch │ RabbitMQ │  MinIO    │
   │  (3 DBs)   │  (Sentinel)   │   (2 nodes)   │ (2 nodes)│ (4 nodes) │
   └────────────────────────────────────────────────────────────────────┘
                                     │
                            ┌────────▼────────┐
                            │ Fanout Worker   │
                            │   (Consumer)    │
                            └─────────────────┘
```

## ✨ Implemented Features

### Core Twitter Functionality
- ✅ **User Management** - Registration, profiles, avatars, follow/unfollow
- ✅ **Tweets** - Create, delete, 280-character limit, threading
- ✅ **Engagement** - Likes, retweets, replies with counters
- ✅ **Timelines** - Home feed, user profiles, pagination
- ✅ **Search** - Full-text search, user search, trending hashtags
- ✅ **Media Upload** - Images and videos via MinIO (10MB limit)
- ✅ **Notifications** - Like, retweet, follow, mention, reply notifications
- ✅ **Smart Fanout** - Optimized distribution (pull-based for >1M followers)

### Infrastructure & DevOps
- ✅ **HTTP/3 (QUIC)** - All services support HTTP/3 with TLS
- ✅ **Microservices** - 7 specialized services with clear boundaries
- ✅ **High Availability** - Multi-replica deployments with failover
- ✅ **Auto-scaling** - HPA configured for dynamic scaling
- ✅ **Service Mesh Ready** - Envoy Gateway integration
- ✅ **Operator-Managed** - CloudNativePG, Redis, ECK, RabbitMQ, MinIO

### Security
- ✅ **Network Policies** - Zero-trust networking between services
- ✅ **Pod Security** - Non-root, read-only filesystem, dropped capabilities
- ✅ **Seccomp Profiles** - Syscall filtering for container security
- ✅ **Rate Limiting** - Redis-based API throttling
- ✅ **CORS Protection** - Cross-origin request handling

## 🚀 Quick Start

### Prerequisites
- Docker
- kubectl
- Helm 3.x
- k3d or kind (for local development)

### 1. Create Kubernetes Cluster

```bash
# Using k3d
k3d cluster create twitter-clone \
  --servers 1 \
  --agents 3 \
  --port "80:80@loadbalancer" \
  --port "443:443@loadbalancer"
```

### 2. Install Operators

```bash
cd twitter-clone/helm/twitter-stack
./scripts/install-operators.sh
```

This installs:
- CloudNativePG (PostgreSQL)
- Spotahome Redis Operator
- ECK (Elasticsearch)
- RabbitMQ Cluster Operator
- MinIO Operator
- Envoy Gateway

### 3. Build and Deploy Services

```bash
# Build all services
make build-all

# Push to registry (update registry in Makefile)
make push-all

# Or for local development with k3d
make import-k3d CLUSTER=twitter-clone
```

### 4. Deploy with Helm

```bash
# Create namespace
kubectl create namespace twitter
kubectl label namespace twitter \
  pod-security.kubernetes.io/enforce=baseline

# Install
helm install twitter ./helm/twitter-stack \
  -n twitter \
  -f ./helm/twitter-stack/values.yaml

# For production
helm install twitter ./helm/twitter-stack \
  -n twitter \
  -f ./helm/twitter-stack/values-prod.yaml
```

## 📡 API Endpoints

### User Service (Port 8080)
| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | `/users` | Create user |
| GET | `/users/{id}` | Get user by ID |
| GET | `/users/username/{username}` | Get user by username |
| PUT | `/users/{id}` | Update user profile |
| POST | `/users/{id}/follow` | Follow user |
| DELETE | `/users/{id}/follow` | Unfollow user |
| GET | `/users/{id}/followers` | Get followers |
| GET | `/users/{id}/following` | Get following |

### Tweet Service (Port 8080)
| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | `/tweets` | Create tweet |
| GET | `/tweets/{id}` | Get tweet |
| DELETE | `/tweets/{id}` | Delete tweet |
| POST | `/tweets/{id}/like` | Like tweet |
| DELETE | `/tweets/{id}/like` | Unlike tweet |
| POST | `/tweets/{id}/retweet` | Retweet |
| DELETE | `/tweets/{id}/retweet` | Unretweet |
| GET | `/users/{id}/tweets` | Get user tweets |

### Timeline Service (Port 8080)
| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/timeline/home` | Get home timeline |
| GET | `/timeline/user/{id}` | Get user timeline |
| POST | `/timeline/add` | Add to timeline |

### Search Service (Port 8080)
| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/search/tweets` | Search tweets |
| GET | `/search/users` | Search users |
| GET | `/trending` | Get trending topics |

### Media Service (Port 8080)
| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | `/media/upload` | Direct upload |
| POST | `/media/presigned` | Get presigned URL |
| GET | `/media/{id}` | Get media info |
| DELETE | `/media/{id}` | Delete media |

### Notification Service (Port 8080)
| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/notifications` | Get notifications |
| GET | `/notifications/unread` | Get unread count |
| PUT | `/notifications/{id}/read` | Mark as read |

## 🔧 Configuration

### Environment Variables
```bash
# Common for all services
TLS_ENABLED=true              # Enable HTTP/3
INSTANCE_ID=<pod-name>        # Auto-set by Kubernetes

# Database URLs (service-specific)
DATABASE_URL=postgres://...   # Set per service

# Redis
REDIS_ADDR=redis-sentinel:6379
REDIS_SENTINEL_ADDRS=sentinel-0:26379,sentinel-1:26379
REDIS_MASTER_NAME=mymaster

# Elasticsearch
ELASTICSEARCH_URL=http://elasticsearch:9200

# RabbitMQ
RABBITMQ_URL=amqp://user:pass@rabbitmq:5672/

# MinIO
MINIO_ENDPOINT=minio:9000
MINIO_ACCESS_KEY=<access-key>
MINIO_SECRET_KEY=<secret-key>
```

### Helm Values

Key configurations in `values.yaml`:
```yaml
replicas:
  userService: 3
  tweetService: 3
  timelineService: 3
  searchService: 2
  mediaService: 2
  notificationService: 2
  fanoutService: 3

resources:
  default:
    requests:
      cpu: 100m
      memory: 128Mi
    limits:
      cpu: 500m
      memory: 512Mi
```

## 🧪 Testing

### Run Tests
```bash
# Unit tests
make test

# Integration tests
make test-integration

# Helm tests
helm test twitter -n twitter

# Security audit
./helm/twitter-stack/scripts/run-security-audit.sh
```

### Load Testing
```bash
# Install k6
brew install k6

# Run load tests
k6 run scripts/load-test.js
```

## 📊 Monitoring

### Check Service Health
```bash
# All services status
kubectl get pods -n twitter

# Service logs
kubectl logs -n twitter -l app=user-service --tail=50

# Specific service
kubectl port-forward -n twitter svc/user-service 8080:8080
curl http://localhost:8080/health
```

### Database Status
```bash
# PostgreSQL clusters
kubectl get clusters.postgresql.cnpg.io -n twitter

# Redis
kubectl get redisfailovers -n twitter

# Elasticsearch
kubectl get elasticsearch -n twitter

# RabbitMQ
kubectl get rabbitmqclusters -n twitter
```

## 🔍 Architecture Decisions

### Why Microservices?
- **Independent scaling** - Scale tweet service separately from users
- **Technology flexibility** - Use best tool for each job
- **Fault isolation** - One service failure doesn't bring down the system
- **Team autonomy** - Teams can work independently

### Smart Fanout Strategy
```
< 10K followers     → Push to all timelines
10K - 1M followers  → Push to active users only  
> 1M followers      → Pull-based (no fanout)
```

### Data Storage Strategy
- **PostgreSQL** - Transactional data (users, tweets, notifications)
- **Redis** - Timelines, caching, real-time counters
- **Elasticsearch** - Full-text search, trending analysis
- **MinIO** - Media storage with S3-compatible API
- **RabbitMQ** - Async job processing, event distribution

## 🚧 Roadmap / Missing Features

### High Priority
- [ ] JWT Authentication system
- [ ] Direct Messages (DMs)
- [ ] WebSocket for real-time updates
- [ ] User blocking/muting

### Medium Priority  
- [ ] Bookmarks
- [ ] Lists
- [ ] Advanced analytics
- [ ] Web frontend (React/Vue)

### Low Priority
- [ ] Spaces (audio rooms)
- [ ] Admin dashboard
- [ ] Monetization features
- [ ] AI recommendations

## 🤝 Contributing

1. Fork the repository
2. Create feature branch (`git checkout -b feature/amazing`)
3. Commit changes (`git commit -m 'Add amazing feature'`)
4. Push to branch (`git push origin feature/amazing`)
5. Open Pull Request

## 📝 License

MIT License - See LICENSE file for details

## 🙏 Acknowledgments

- Built with Go 1.22
- Uses QUIC-Go for HTTP/3 support
- Kubernetes operators for infrastructure management
- Inspired by Twitter's original architecture papers