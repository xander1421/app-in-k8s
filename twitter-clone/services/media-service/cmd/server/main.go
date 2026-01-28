package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/google/uuid"

	"github.com/alexprut/twitter-clone/pkg/middleware"
	"github.com/alexprut/twitter-clone/pkg/queue"
	"github.com/alexprut/twitter-clone/pkg/server"
	"github.com/alexprut/twitter-clone/pkg/storage"
	"github.com/alexprut/twitter-clone/services/media-service/internal/handlers"
	"github.com/alexprut/twitter-clone/services/media-service/internal/service"
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	instanceID := os.Getenv("INSTANCE_ID")
	if instanceID == "" {
		instanceID = uuid.New().String()[:8]
	}

	log.Printf("Starting media-service instance: %s", instanceID)

	// MinIO connection (required)
	minioEndpoint := os.Getenv("MINIO_ENDPOINT")
	if minioEndpoint == "" {
		minioEndpoint = "localhost:9000"
	}
	minioAccessKey := os.Getenv("MINIO_ACCESS_KEY")
	if minioAccessKey == "" {
		minioAccessKey = "minioadmin"
	}
	minioSecretKey := os.Getenv("MINIO_SECRET_KEY")
	if minioSecretKey == "" {
		minioSecretKey = "minioadmin"
	}
	minioUseSSL := os.Getenv("MINIO_USE_SSL") == "true"

	storageClient, err := storage.NewMinIOClient(ctx, minioEndpoint, minioAccessKey, minioSecretKey, minioUseSSL)
	if err != nil {
		log.Fatalf("Failed to connect to MinIO: %v", err)
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

	// Initialize service and handlers
	svc := service.NewMediaService(storageClient, rmq, minioEndpoint)
	handler := handlers.NewMediaHandler(svc)

	// Setup HTTP router
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)
	mux.HandleFunc("GET /health", handlers.HealthHandler)
	mux.HandleFunc("GET /ready", handlers.ReadyHandler)

	// Apply middleware
	var h http.Handler = mux
	h = middleware.Auth(h)
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
		log.Printf("Starting media-service with HTTP/3 on %s", addr)
		if err := srv.ListenAndServe(); err != nil {
			log.Fatalf("Server error: %v", err)
		}
	} else {
		log.Printf("Starting media-service with HTTP/1.1 on %s (no TLS)", addr)
		if err := srv.ListenAndServeInsecure(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server error: %v", err)
		}
	}
}
