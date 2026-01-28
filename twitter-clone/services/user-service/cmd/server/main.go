package main

import (
	"context"
	"crypto/tls"
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
	"github.com/alexprut/twitter-clone/pkg/database"
	"github.com/alexprut/twitter-clone/pkg/middleware"
	"github.com/alexprut/twitter-clone/pkg/moderation"
	"github.com/alexprut/twitter-clone/pkg/queue"
	"github.com/alexprut/twitter-clone/pkg/search"
	"github.com/alexprut/twitter-clone/pkg/server"
	"github.com/alexprut/twitter-clone/services/user-service/internal/handlers"
	"github.com/alexprut/twitter-clone/services/user-service/internal/repository"
	"github.com/alexprut/twitter-clone/services/user-service/internal/service"
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	instanceID := os.Getenv("INSTANCE_ID")
	if instanceID == "" {
		instanceID = uuid.New().String()[:8]
	}

	log.Printf("Starting user-service instance: %s", instanceID)

	var db *database.PostgresDB
	
	// Database connection (optional for frontend testing)
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		dbURL = "postgres://postgres:postgres@localhost:5432/users_db?sslmode=disable"
	}
	
	// Try to connect to database, but don't fail if unavailable (for frontend testing)
	if os.Getenv("SKIP_DB") != "true" {
		var err error
		db, err = database.NewPostgresDB(ctx, dbURL)
		if err != nil {
			log.Printf("Warning: Failed to connect to database: %v", err)
			log.Printf("Running in frontend-only mode. Set SKIP_DB=true to suppress this warning.")
			db = nil
		} else {
			defer db.Close()
		}
	}

	// Initialize repositories (with nil check for frontend-only mode)
	var baseRepo *repository.UserRepository
	var authRepo *repository.UserRepositoryAuth
	
	if db != nil {
		baseRepo = repository.NewUserRepository(db.Pool())
		authRepo = repository.NewUserRepositoryAuth(db.Pool())
		
		// Run migrations
		if err := baseRepo.Migrate(ctx); err != nil {
			log.Fatalf("Failed to run migrations: %v", err)
		}
	} else {
		log.Println("Running in frontend-only mode - database features disabled")
	}

	// Redis connection (optional)
	var redisCache *cache.RedisCache
	redisAddr := os.Getenv("REDIS_ADDR")
	if redisAddr != "" {
		sentinelAddrs := strings.Split(os.Getenv("REDIS_SENTINEL_ADDRS"), ",")
		masterName := os.Getenv("REDIS_MASTER_NAME")
		redisPassword := os.Getenv("REDIS_PASSWORD")
		var err error

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
		var err error
		esClient, err = search.NewElasticsearchClient(esURL)
		if err != nil {
			log.Printf("Warning: Failed to connect to Elasticsearch: %v", err)
		}
	}

	// RabbitMQ connection (optional)
	var rmq *queue.RabbitMQ
	rmqURL := os.Getenv("RABBITMQ_URL")
	if rmqURL != "" {
		var err error
		rmq, err = queue.NewRabbitMQ(rmqURL, instanceID)
		if err != nil {
			log.Printf("Warning: Failed to connect to RabbitMQ: %v", err)
		} else {
			defer rmq.Close()
		}
	}

	// Initialize JWT manager
	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		jwtSecret = "development-secret-change-in-production"
	}
	jwtManager := auth.NewJWTManager(
		[]byte(jwtSecret),
		15*time.Minute,  // access token expiry
		7*24*time.Hour,  // refresh token expiry
		"twitter-clone",
	)
	
	// Initialize content moderator
	moderator := moderation.NewContentModerator(redisCache)
	
	// Initialize services
	svc := service.NewUserService(baseRepo, redisCache, esClient, rmq)
	authSvc := service.NewAuthService(authRepo, jwtManager, moderator)
	
	// Initialize handlers
	handler := handlers.NewUserHandler(svc)
	_ = authSvc // TODO: Use auth service when implementing auth handlers

	// Setup HTTP router
	mux := http.NewServeMux()
	
	// Register user routes
	handler.RegisterRoutes(mux)
	
	// Health check routes
	mux.HandleFunc("GET /health", handlers.HealthHandler)
	mux.HandleFunc("GET /ready", handlers.ReadyHandler)
	
	// WebTransport endpoint
	mux.HandleFunc("GET /webtransport", func(w http.ResponseWriter, r *http.Request) {
		// WebTransport upgrades are handled by the underlying HTTP/3 WebTransport server
		w.WriteHeader(http.StatusOK)
	})

	// Apply middleware
	var h http.Handler = mux
	h = middleware.JWTAuth(jwtManager)(h)  // Use JWT authentication
	h = middleware.RateLimit(100, 1*time.Minute)(h)  // 100 requests per minute
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
		// Try to load proper certificate first
		cert, err := tls.LoadX509KeyPair("certs/cert.pem", "certs/key.pem")
		var tlsConfig *tls.Config
		
		if err != nil {
			log.Printf("Failed to load certificate files, generating self-signed: %v", err)
			tlsConfig, err = server.GenerateSelfSignedCert()
			if err != nil {
				log.Fatalf("Failed to generate TLS cert: %v", err)
			}
		} else {
			log.Println("Using proper certificate files for HTTP/3")
			tlsConfig = &tls.Config{
				Certificates: []tls.Certificate{cert},
				NextProtos:   []string{"h3", "h2", "http/1.1"}, // HTTP/3 first
				MinVersion:   tls.VersionTLS12,
				MaxVersion:   tls.VersionTLS13, // QUIC requires TLS 1.3
			}
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
		log.Printf("Starting user-service with HTTP/3 on %s", addr)
		if err := srv.ListenAndServe(); err != nil {
			log.Fatalf("Server error: %v", err)
		}
	} else {
		log.Printf("Starting user-service with HTTP/1.1 on %s (no TLS)", addr)
		if err := srv.ListenAndServeInsecure(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server error: %v", err)
		}
	}
}
