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
