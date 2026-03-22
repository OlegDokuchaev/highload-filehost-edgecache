package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"time"

	"origin-service/config"
	"origin-service/handlers"
	"origin-service/logger"
	"origin-service/repository"
	"origin-service/service"
)

func main() {
	cfg, err := config.Load("config.yml")
	if err != nil {
		slog.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	if _, err := logger.Init(&cfg.Logging); err != nil {
		slog.Error("failed to init logger", "error", err)
		os.Exit(1)
	}

	// port := getEnv("APP_PORT", cfg.App.Port)
	// minioEndpoint := getEnv("MINIO_ENDPOINT", cfg.App.MinioEndpoint)
	// minioAccessKey := getEnv("MINIO_ACCESS_KEY", cfg.App.MinioAccessKey)
	// minioSecretKey := getEnv("MINIO_SECRET_KEY", cfg.App.MinioSecretKey)
	// minioBucket := getEnv("MINIO_BUCKET", cfg.App.MinioBucket)
	// minioUseSSL := getEnvBool("MINIO_USE_SSL", cfg.App.MinioUseSSL)
	// downloadBaseURL := getEnv("DOWNLOAD_BASE_URL", cfg.App.DownloadBaseURL)
	// downloadURLExpiry := getEnvInt("DOWNLOAD_URL_EXPIRY", cfg.App.DownloadURLExpiry)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	db, err := repository.NewMinIOStorage(
		ctx,
		cfg.App.MinioEndpoint,
		cfg.App.MinioAccessKey,
		cfg.App.MinioSecretKey,
		cfg.App.MinioBucket,
		cfg.App.MinioUseSSL,
	)
	if err != nil {
		slog.Error("storage init failed", "error", err)
		os.Exit(1)
	}
	fileRepo := repository.NewFileRepository(db)
	uploadService := service.NewUploadService(fileRepo)
	tokenVerifier := handlers.NewStubTokenVerifier()
	urlHandler := handlers.NewURLHandler(
		uploadService,
		tokenVerifier,
		cfg.App.DownloadBaseURL,
		cfg.App.DownloadURLExpiry,
	)

	mux := http.NewServeMux()
	urlHandler.Register(mux)

	slog.Info("origin-service starting", "port", cfg.App.Port, "minio_endpoint", cfg.App.MinioEndpoint, "minio_bucket", cfg.App.MinioBucket)

	mux.HandleFunc("/health", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	if err = http.ListenAndServe(":"+cfg.App.Port, mux); err != nil {
		slog.Error("server failed", "error", err)
		os.Exit(1)
	}
}

// func getEnv(key, fallback string) string {
// 	if value := os.Getenv(key); value != "" {
// 		return value
// 	}
// 	return fallback
// }

// func getEnvBool(key string, fallback bool) bool {
// 	if value := os.Getenv(key); value != "" {
// 		return value == "true"
// 	}
// 	return fallback
// }

// func getEnvInt(key string, fallback int) int {
// 	if value := os.Getenv(key); value != "" {
// 		if i, err := strconv.Atoi(value); err == nil {
// 			return i
// 		}
// 	}
// 	return fallback
// }