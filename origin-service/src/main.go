package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"time"

	service "origin-service/service"
	"origin-service/handlers"
	"origin-service/repository"
)

func main() {
	port := getEnv("APP_PORT", "8080")
	minioEndpoint := getEnv("MINIO_ENDPOINT", "minio:9000")
	minioAccessKey := getEnv("MINIO_ACCESS_KEY", "minioadmin")
	minioSecretKey := getEnv("MINIO_SECRET_KEY", "minioadmin")
	minioBucket := getEnv("MINIO_BUCKET", "origin-bucket")
	minioUseSSL := getEnv("MINIO_USE_SSL", "false") == "true"

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	db, err := repository.NewMinIOStorage(
		ctx,
		minioEndpoint,
		minioAccessKey,
		minioSecretKey,
		minioBucket,
		minioUseSSL,
	)
	if err != nil {
		log.Fatalf("storage init failed: %v", err)
	}
	fileRepo := repository.NewFileRepository(db)
	uploadService := service.NewUploadService(fileRepo)
	tokenVerifier := handlers.NewStubTokenVerifier()
	urlHandler := handlers.NewURLHandler(
		uploadService,
		tokenVerifier,
		getEnv("DOWNLOAD_BASE_URL", "http://localhost:"+port),
		3600,
	)

	mux := http.NewServeMux()
	urlHandler.Register(mux)

	log.Printf("origin-service starting on :%s", port)
	log.Printf("MinIO endpoint: %s, bucket: %s", minioEndpoint, minioBucket)

	mux.HandleFunc("/health", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	if err = http.ListenAndServe(":"+port, mux); err != nil {
		log.Fatalf("server failed: %v", err)
	}
}

func getEnv(key, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	return value
}
