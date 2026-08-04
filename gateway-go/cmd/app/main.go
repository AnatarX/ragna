package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"gateway-go/internal/cache"
	deliveryHTTP "gateway-go/internal/delivery/http"
	"gateway-go/internal/grpcclient"

	"github.com/redis/go-redis/v9"
)

func main() {
	// 1. Считываем переменные окружения с дефолтами
	redisAddr := getEnv("REDIS_ADDR", "localhost:6379")
	mlGrpcAddr := getEnv("ML_GRPC_ADDR", "localhost:50051")
	httpPort := getEnv("HTTP_PORT", "8080")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 2. Инициализируем Redis-клиент и Семантический Кэш
	rdb := redis.NewClient(&redis.Options{
		Addr: redisAddr,
	})
	if err := rdb.Ping(ctx).Err(); err != nil {
		log.Printf("[WARN] Failed to ping Redis at %s: %v (cache may be unavailable)", redisAddr, err)
	} else {
		log.Printf("[INFO] Connected to Redis at %s", redisAddr)
	}

	semanticCache := cache.NewSemanticCache(rdb)

	// 3. Подключаем gRPC-клиент к Python ML сервису
	mlClient, err := grpcclient.New(mlGrpcAddr)
	if err != nil {
		log.Fatalf("[FATAL] Failed to initialize gRPC client: %v", err)
	}
	defer mlClient.Close()
	log.Printf("[INFO] Connected to ML gRPC service at %s", mlGrpcAddr)

	// 4. Регистрируем роуты и хэндлеры
	handler := deliveryHTTP.NewHandler(mlClient, semanticCache)

	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/query/stream", handler.StreamQuery)

	srv := &http.Server{
		Addr:    ":" + httpPort,
		Handler: mux,
	}

	// 5. Graceful Shutdown
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	go func() {
		log.Printf("[INFO] Starting HTTP Gateway on port :%s...", httpPort)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("[FATAL] HTTP server error: %v", err)
		}
	}()

	<-stop
	log.Println("[INFO] Shutting down HTTP Gateway gracefully...")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("[ERROR] Server forced to shutdown: %v", err)
	}

	log.Println("[INFO] Server stopped.")
}

func getEnv(key, defaultValue string) string {
	if val, exists := os.LookupEnv(key); exists && val != "" {
		return val
	}
	return defaultValue
}
