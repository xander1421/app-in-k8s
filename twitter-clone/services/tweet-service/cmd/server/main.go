package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/google/uuid"

	"github.com/alexprut/twitter-clone/pkg/auth"
	"github.com/alexprut/twitter-clone/pkg/cache"
	"github.com/alexprut/twitter-clone/pkg/clients"
	"github.com/alexprut/twitter-clone/pkg/database"
	"github.com/alexprut/twitter-clone/pkg/middleware"
	"github.com/alexprut/twitter-clone/pkg/queue"
	"github.com/alexprut/twitter-clone/pkg/search"
	"github.com/alexprut/twitter-clone/pkg/server"
	"github.com/alexprut/twitter-clone/services/tweet-service/internal/handlers"
	"github.com/alexprut/twitter-clone/services/tweet-service/internal/repository"
	"github.com/alexprut/twitter-clone/services/tweet-service/internal/service"
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	instanceID := os.Getenv("INSTANCE_ID")
	if instanceID == "" {
		instanceID = uuid.New().String()[:8]
	}

	log.Printf("Starting tweet-service instance: %s", instanceID)

	// Database connection
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		dbURL = "postgres://postgres:postgres@localhost:5432/tweets_db?sslmode=disable"
	}

	db, err := database.NewPostgresDB(ctx, dbURL)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	// Initialize repository and run migrations
	repo := repository.NewTweetRepository(db.Pool())
	if err := repo.Migrate(ctx); err != nil {
		log.Fatalf("Failed to run migrations: %v", err)
	}
	// Create bookmarks table
	if err := repo.CreateBookmarksTable(ctx); err != nil {
		log.Printf("Warning: Failed to create bookmarks table: %v", err)
	}

	// Redis connection (optional)
	var redisCache *cache.RedisCache
	redisAddr := os.Getenv("REDIS_ADDR")
	if redisAddr != "" {
		sentinelAddrs := strings.Split(os.Getenv("REDIS_SENTINEL_ADDRS"), ",")
		masterName := os.Getenv("REDIS_MASTER_NAME")
		redisPassword := os.Getenv("REDIS_PASSWORD")

		if masterName != "" && len(sentinelAddrs) > 0 && sentinelAddrs[0] != "" {
			redisCache, err = cache.NewRedisCache(ctx, sentinelAddrs, masterName, redisPassword, instanceID)
		} else {
			redisCache, err = cache.NewRedisCacheSimple(ctx, redisAddr, redisPassword, instanceID)
		}
		if err != nil {
			log.Printf("Warning: Failed to connect to Redis: %v", err)
		} else {
			defer redisCache.Close()
		}
	}

	// Elasticsearch connection (optional)
	var esClient *search.ElasticsearchClient
	esURL := os.Getenv("ELASTICSEARCH_URL")
	if esURL != "" {
		esClient, err = search.NewElasticsearchClient(esURL)
		if err != nil {
			log.Printf("Warning: Failed to connect to Elasticsearch: %v", err)
		}
	}

	// RabbitMQ connection (optional)
	var rmq *queue.RabbitMQ
	rmqURL := os.Getenv("RABBITMQ_URL")
	if rmqURL != "" {
		rmq, err = queue.NewRabbitMQ(rmqURL, instanceID)
		if err != nil {
			log.Printf("Warning: Failed to connect to RabbitMQ: %v", err)
		} else {
			defer rmq.Close()
		}
	}

	// User service client (optional)
	var userClient *clients.UserServiceClient
	userServiceURL := os.Getenv("USER_SERVICE_URL")
	if userServiceURL == "" {
		userServiceURL = "http://user-service:8080"
	}
	userClient = clients.NewUserServiceClient(userServiceURL)

	// Initialize JWT manager (for middleware)
	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		jwtSecret = "development-secret-change-in-production"
	}
	jwtManager := auth.NewJWTManager(
		[]byte(jwtSecret),
		15*time.Minute,
		7*24*time.Hour,
		"twitter-clone",
	)

	// Initialize service and handlers
	svc := service.NewTweetService(repo, redisCache, esClient, rmq, userClient)
	handler := handlers.NewTweetHandler(svc)

	// Setup HTTP router
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)
	handler.RegisterBookmarkRoutes(mux)  // Register bookmark endpoints
	mux.HandleFunc("GET /health", handlers.HealthHandler)
	mux.HandleFunc("GET /ready", handlers.ReadyHandler)

	// Apply middleware
	var h http.Handler = mux
	h = middleware.JWTAuth(jwtManager)(h)  // Use JWT authentication
	h = middleware.RateLimit(300, 1*time.Minute)(h)  // 300 requests per minute for tweets
	h = middleware.CORS(h)
	h = middleware.Logger(h)
	h = middleware.Recovery(h)

	// Start server
	addr := os.Getenv("ADDR")
	if addr == "" {
		addr = ":8080"
	}

	useTLS := os.Getenv("TLS_ENABLED") == "true"

	var srv *server.Server
	if useTLS {
		tlsConfig, err := server.GenerateSelfSignedCert()
		if err != nil {
			log.Fatalf("Failed to generate TLS cert: %v", err)
		}
		srv = server.NewServer(addr, h, tlsConfig)
	} else {
		srv = server.NewServer(addr, h, nil)
	}

	// Graceful shutdown
	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		<-sigCh

		log.Println("Shutting down...")
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer shutdownCancel()

		if err := srv.Shutdown(shutdownCtx); err != nil {
			log.Printf("Shutdown error: %v", err)
		}
		cancel()
	}()

	// Start server
	if useTLS {
		log.Printf("Starting tweet-service with HTTP/3 on %s", addr)
		if err := srv.ListenAndServe(); err != nil {
			log.Fatalf("Server error: %v", err)
		}
	} else {
		log.Printf("Starting tweet-service with HTTP/1.1 on %s (no TLS)", addr)
		if err := srv.ListenAndServeInsecure(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server error: %v", err)
		}
	}
}
