package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/alexprut/fileshare/internal/cache"
	"github.com/alexprut/fileshare/internal/config"
	"github.com/alexprut/fileshare/internal/database"
	"github.com/alexprut/fileshare/internal/handlers"
	"github.com/alexprut/fileshare/internal/models"
	"github.com/alexprut/fileshare/internal/queue"
	"github.com/alexprut/fileshare/internal/search"
	"github.com/alexprut/fileshare/internal/server"
)

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	cfg := config.Load()
	log.Printf("Starting FileShare server [instance: %s]", cfg.InstanceID)
	log.Printf("Environment: %s, Port: %s", cfg.Environment, cfg.Port)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// ============== PostgreSQL ==============
	log.Println("Connecting to PostgreSQL...")
	db, err := database.NewPostgresDB(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("PostgreSQL connection failed: %v", err)
	}
	defer db.Close()
	log.Println("PostgreSQL connected and migrated")

	// ============== Redis (Sentinel) ==============
	var redisCache *cache.RedisCache
	log.Printf("Connecting to Redis Sentinel at %v...", cfg.RedisSentinelAddrs)
	redisCache, err = cache.NewRedisCache(ctx, cfg.RedisSentinelAddrs, cfg.RedisMasterName, cfg.RedisPassword, cfg.InstanceID)
	if err != nil {
		log.Printf("Redis connection failed (cluster features disabled): %v", err)
	} else {
		defer redisCache.Close()
		log.Println("Redis Sentinel connected")

		// Register for file events from other instances
		redisCache.OnFileEvent(func(event models.FileEvent) {
			if event.InstanceID != cfg.InstanceID {
				log.Printf("[CLUSTER EVENT] %s: file %s (%s) from instance %s",
					event.Type, event.FileName, event.FileID[:8], event.InstanceID[:8])
			}
		})
	}

	// ============== Elasticsearch ==============
	var esClient *search.ElasticsearchClient
	log.Printf("Connecting to Elasticsearch at %s...", cfg.ElasticsearchURL)
	esClient, err = search.NewElasticsearchClient(cfg.ElasticsearchURL)
	if err != nil {
		log.Printf("Elasticsearch connection failed (search disabled): %v", err)
	} else {
		log.Println("Elasticsearch connected")
	}

	// ============== RabbitMQ ==============
	var rmq *queue.RabbitMQ
	log.Printf("Connecting to RabbitMQ at %s...", cfg.RabbitMQURL)
	rmq, err = queue.NewRabbitMQ(cfg.RabbitMQURL, cfg.InstanceID)
	if err != nil {
		log.Printf("RabbitMQ connection failed (queues disabled): %v", err)
	} else {
		log.Println("RabbitMQ connected")

		// Register job handlers
		rmq.RegisterHandler(queue.QueueProcessing, func(job models.Job) error {
			log.Printf("[JOB] Processing file %s", job.FileID)
			// Simulate processing
			time.Sleep(100 * time.Millisecond)
			return nil
		})

		rmq.RegisterHandler(queue.QueueThumbnails, func(job models.Job) error {
			log.Printf("[JOB] Generating thumbnail for %s", job.FileID)
			// Simulate thumbnail generation
			time.Sleep(200 * time.Millisecond)
			return nil
		})

		rmq.RegisterHandler(queue.QueueNotify, func(job models.Job) error {
			log.Printf("[JOB] Sending notification: %v", job.Payload)
			return nil
		})

		// Start consumers
		if err := rmq.StartAllConsumers(ctx); err != nil {
			log.Printf("Failed to start consumers: %v", err)
		}

		defer rmq.Close()
	}

	// ============== HTTP Handlers ==============
	h := handlers.NewHandlers(cfg, db, redisCache, esClient, rmq)

	// ============== Server ==============
	addr := ":" + cfg.Port
	useTLS := os.Getenv("TLS_ENABLED") == "true"

	var srv *server.Server
	if useTLS {
		tlsConfig, err := server.GenerateSelfSignedCert()
		if err != nil {
			log.Fatalf("TLS config error: %v", err)
		}
		srv = server.NewServer(addr, h.Router(), tlsConfig)
	} else {
		srv = server.NewServer(addr, h.Router(), nil)
	}

	// Start server in goroutine
	go func() {
		var err error
		if useTLS {
			log.Printf("Starting HTTP/3 + HTTP/2 server on %s (TLS)", addr)
			err = srv.ListenAndServe()
		} else {
			log.Printf("Starting HTTP/1.1 server on %s (no TLS - dev mode)", addr)
			err = srv.ListenAndServeInsecure()
		}
		if err != nil {
			log.Printf("Server error: %v", err)
		}
	}()

	log.Println("Server started successfully")
	log.Println("Endpoints:")
	log.Println("  GET  /health         - Liveness probe")
	log.Println("  GET  /health/ready   - Readiness probe (checks all services)")
	log.Println("  GET  /cluster        - Cluster info (active peers)")
	log.Println("  POST /files          - Upload file")
	log.Println("  GET  /files          - List files")
	log.Println("  GET  /files/{id}     - Get file metadata")
	log.Println("  GET  /files/{id}/download - Download file")
	log.Println("  DELETE /files/{id}   - Delete file")
	log.Println("  GET  /search?q=      - Search files (Elasticsearch)")
	log.Println("  POST /files/{id}/share - Create share link")
	log.Println("  GET  /share/{token}  - Access shared file")
	log.Println("  GET  /queues         - Queue statistics (RabbitMQ)")

	// Wait for shutdown signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down...")
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("Server shutdown error: %v", err)
	}

	log.Println("Server stopped")
}
