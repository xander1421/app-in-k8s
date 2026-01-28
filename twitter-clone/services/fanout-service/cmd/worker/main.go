package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/google/uuid"

	"github.com/alexprut/twitter-clone/pkg/cache"
	"github.com/alexprut/twitter-clone/pkg/clients"
	"github.com/alexprut/twitter-clone/pkg/models"
	"github.com/alexprut/twitter-clone/pkg/queue"
	"github.com/alexprut/twitter-clone/pkg/search"
	"github.com/alexprut/twitter-clone/services/fanout-service/internal/service"
)

// simpleNotificationService implements the NotificationService interface
type simpleNotificationService struct {
	client *clients.NotificationServiceClient
}

func (s *simpleNotificationService) SendPushNotification(ctx context.Context, userID string, notification *models.Notification) error {
	return s.client.SendNotification(ctx, notification)
}

func (s *simpleNotificationService) SendEmailNotification(ctx context.Context, userID string, notification *models.Notification) error {
	// For now, use the same method
	return s.client.SendNotification(ctx, notification)
}

func (s *simpleNotificationService) SendSSENotification(ctx context.Context, userID string, notification *models.Notification) error {
	// For now, use the same method  
	return s.client.SendNotification(ctx, notification)
}

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	instanceID := os.Getenv("INSTANCE_ID")
	if instanceID == "" {
		instanceID = uuid.New().String()[:8]
	}

	log.Printf("Starting fanout-service worker instance: %s", instanceID)

	// RabbitMQ connection (required)
	rmqURL := os.Getenv("RABBITMQ_URL")
	if rmqURL == "" {
		rmqURL = "amqp://guest:guest@localhost:5672/"
	}

	rmq, err := queue.NewRabbitMQ(rmqURL, instanceID)
	if err != nil {
		log.Fatalf("Failed to connect to RabbitMQ: %v", err)
	}
	defer rmq.Close()

	// Redis connection (required for timeline operations)
	var redisCache *cache.RedisCache
	redisAddr := os.Getenv("REDIS_ADDR")
	if redisAddr == "" {
		redisAddr = "localhost:6379"
	}

	sentinelAddrs := strings.Split(os.Getenv("REDIS_SENTINEL_ADDRS"), ",")
	masterName := os.Getenv("REDIS_MASTER_NAME")
	redisPassword := os.Getenv("REDIS_PASSWORD")

	if masterName != "" && len(sentinelAddrs) > 0 && sentinelAddrs[0] != "" {
		redisCache, err = cache.NewRedisCache(ctx, sentinelAddrs, masterName, redisPassword, instanceID)
	} else {
		redisCache, err = cache.NewRedisCacheSimple(ctx, redisAddr, redisPassword, instanceID)
	}
	if err != nil {
		log.Fatalf("Failed to connect to Redis: %v", err)
	}
	defer redisCache.Close()

	// Service clients
	userServiceURL := os.Getenv("USER_SERVICE_URL")
	if userServiceURL == "" {
		userServiceURL = "http://user-service:8080"
	}
	userClient := clients.NewUserClient(userServiceURL)

	tweetServiceURL := os.Getenv("TWEET_SERVICE_URL")
	if tweetServiceURL == "" {
		tweetServiceURL = "http://tweet-service:8080"
	}
	tweetClient := clients.NewTweetClient(tweetServiceURL)

	notificationURL := os.Getenv("NOTIFICATION_SERVICE_URL")
	if notificationURL == "" {
		notificationURL = "http://notification-service:8080"
	}
	notificationClient := clients.NewNotificationClient(notificationURL)

	// Elasticsearch client for search indexing
	var esClient *search.ElasticsearchClient
	esURL := os.Getenv("ELASTICSEARCH_URL")
	if esURL != "" {
		esClient, err = search.NewElasticsearchClient(esURL)
		if err != nil {
			log.Printf("Warning: Failed to connect to Elasticsearch: %v", err)
		}
	}

	// Create a simple notification service wrapper
	notificationSvc := &simpleNotificationService{client: notificationClient}
	
	// Initialize complete fanout service
	svc := service.NewFanoutServiceComplete(redisCache, userClient, tweetClient, esClient, notificationSvc)

	// Register handlers for each queue
	rmq.RegisterHandler(queue.QueueFanoutHigh, func(job models.FanoutJob) error {
		return svc.ProcessTweetFanout(ctx, job)
	})

	rmq.RegisterHandler(queue.QueueFanoutNormal, func(job models.FanoutJob) error {
		return svc.ProcessTweetFanout(ctx, job)
	})

	rmq.RegisterHandler(queue.QueueSearchIndex, func(job models.FanoutJob) error {
		return svc.ProcessSearchIndex(ctx, job)
	})

	rmq.RegisterHandler(queue.QueueNotifyPush, func(job models.FanoutJob) error {
		return svc.ProcessNotification(ctx, job)
	})

	// Start all consumers
	if err := rmq.StartAllConsumers(ctx); err != nil {
		log.Fatalf("Failed to start consumers: %v", err)
	}

	// Start health check server for Kubernetes probes
	healthAddr := os.Getenv("HEALTH_ADDR")
	if healthAddr == "" {
		healthAddr = ":8080"
	}
	go func() {
		mux := http.NewServeMux()
		mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"status":"healthy"}`))
		})
		mux.HandleFunc("GET /ready", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"status":"ready"}`))
		})
		log.Printf("Starting health server on %s", healthAddr)
		if err := http.ListenAndServe(healthAddr, mux); err != nil && err != http.ErrServerClosed {
			log.Printf("Health server error: %v", err)
		}
	}()

	log.Println("Fanout worker started. Waiting for jobs...")

	// Wait for shutdown signal
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	log.Println("Shutting down fanout worker...")
	cancel()
}
