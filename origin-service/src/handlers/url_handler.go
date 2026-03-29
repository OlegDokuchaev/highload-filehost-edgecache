package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"

	service "origin-service/service"
)

type TokenVerifierInterface interface {
	Verify(ctx context.Context, authHeader string) (string, error)
}

type URLHandler struct {
	uploadService *service.UploadService
	tokenVerifier TokenVerifierInterface
	downloadBase  string
	cacheMaxAge   int
}

func NewURLHandler(
	uploadService *service.UploadService,
	tokenVerifier TokenVerifierInterface,
	downloadBase string,
	cacheMaxAge int,
) *URLHandler {
	return &URLHandler{
		uploadService: uploadService,
		tokenVerifier: tokenVerifier,
		downloadBase:  strings.TrimRight(downloadBase, "/"),
		cacheMaxAge:   cacheMaxAge,
	}
}

func (h *URLHandler) Register(mux *http.ServeMux) {
	mux.HandleFunc("/files", h.postFiles)
	mux.HandleFunc("/users/me/files", h.getMyFiles)
	mux.HandleFunc("/files/", h.getFileByID)
}

func (h *URLHandler) postFiles(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		slog.Error("method not allowed", "method", r.Method)
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		slog.Error("missing file", "error", err)
		writeError(w, http.StatusBadRequest, "missing file")
		return
	}
	defer file.Close()

	userID, err := h.tokenVerifier.Verify(r.Context(), r.Header.Get("Authorization"))
	if err != nil {
		slog.Error("token invalid", "error", err)
		writeError(w, http.StatusUnauthorized, "token invalid")
		return
	}

	contentType := header.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	result, err := h.uploadService.Upload(r.Context(), service.UploadInput{
		UserID:      userID,
		FileName:    header.Filename,
		ContentType: contentType,
		SizeBytes:   header.Size,
		Body:        file,
	})
	if err != nil {
		if errors.Is(err, service.ErrPayloadTooLarge) {
			slog.Error("payload too large", "error", err)
			writeError(w, http.StatusRequestEntityTooLarge, "payload too large")
			return
		}
		slog.Error("failed to upload file", "error", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	writeJSON(w, http.StatusCreated, map[string]any{
		"file_id":       result.FileID,
		"owner_user_id": result.OwnerUserID,
		"filename":      result.FileName,
		"content_type":  result.ContentType,
		"size_bytes":    result.SizeBytes,
		"created_at":    result.CreatedAt,
		"download_url":  h.buildDownloadURL(result.FileID),
	})
	slog.Info("file uploaded", "fileID", result.FileID, "ownerUserID", result.OwnerUserID, "fileName", result.FileName, "contentType", result.ContentType, "sizeBytes", result.SizeBytes, "createdAt", result.CreatedAt)
}

func (h *URLHandler) getMyFiles(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		slog.Error("method not allowed", "method", r.Method)
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	userID, err := h.tokenVerifier.Verify(r.Context(), r.Header.Get("Authorization"))
	if err != nil {
		slog.Error("token invalid", "error", err)
		writeError(w, http.StatusUnauthorized, "token invalid")
		return
	}

	items, err := h.uploadService.ListByUserID(userID)
	if err != nil {
		slog.Error("failed to list files", "error", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	payloadItems := make([]map[string]any, 0, len(items))
	for _, item := range items {
		payloadItems = append(payloadItems, map[string]any{
			"file_id":      item.FileID,
			"filename":     item.FileName,
			"content_type": item.ContentType,
			"size_bytes":   item.SizeBytes,
			"created_at":   item.CreatedAt,
			"download_url": h.buildDownloadURL(item.FileID),
		})
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"items": payloadItems,
		"total": len(payloadItems),
	})
	slog.Info("files listed", "userID", userID, "count", len(items))
}

func (h *URLHandler) getFileByID(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		slog.Error("method not allowed", "method", r.Method)
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	fileID := strings.TrimPrefix(r.URL.Path, "/files/")
	if fileID == "" {
		slog.Warn("get file called with empty fileID")
		writeError(w, http.StatusNotFound, "not found")
		return
	}

	result, err := h.uploadService.DownloadByFileID(r.Context(), fileID)
	if err != nil {
		if errors.Is(err, service.ErrNotFound) {
			slog.Warn("file not found", "fileID", fileID)
			writeError(w, http.StatusNotFound, "not found")
			return
		}
		slog.Error("failed to get file", "error", err, "fileID", fileID)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	defer result.Reader.Close() // обязательно закрываем поток

	// Проверка If-None-Match
	ifNoneMatch := strings.TrimSpace(r.Header.Get("If-None-Match"))
	if matchETag(ifNoneMatch, result.ETag) {
		w.Header().Set("ETag", result.ETag)
		w.Header().Set("Cache-Control", h.cacheControlValue())
		w.WriteHeader(http.StatusNotModified)
		slog.Info("file not modified", "fileID", fileID)
		return
	}

	// Устанавливаем заголовки
	w.Header().Set("Content-Type", result.ContentType)
	w.Header().Set("Cache-Control", h.cacheControlValue())
	w.Header().Set("ETag", result.ETag)
	w.WriteHeader(http.StatusOK)

	// Стриминг содержимого
	if _, err := io.Copy(w, result.Reader); err != nil {
		slog.Error("failed to copy file content", "error", err, "fileID", fileID)
	}
	slog.Info("file downloaded", "fileID", fileID, "contentType", result.ContentType, "sizeBytes", result.Size)
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

func (h *URLHandler) buildDownloadURL(fileID string) string {
	return h.downloadBase + "/files/" + fileID
}

func (h *URLHandler) cacheControlValue() string {
	return fmt.Sprintf("public, max-age=%d", h.cacheMaxAge)
}

// parseETag извлекает значение ETag и признак слабости из строки вида "abc" или W/"abc"
func parseETag(s string) (value string, weak bool) {
	s = strings.TrimSpace(s)
	if strings.HasPrefix(s, "W/") {
		weak = true
		s = s[2:]
	}
	if len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"' {
		value = s[1 : len(s)-1]
	}
	return value, weak
}

// matchETag проверяет, соответствует ли переданный клиентом If-None-Match серверному ETag
func matchETag(ifNoneMatch string, serverETag string) bool {
	if ifNoneMatch == "" {
		return false
	}
	if ifNoneMatch == "*" {
		return true
	}
	// Заголовок может содержать несколько ETag через запятую
	slog.Info("ETag comparison 1", "If-None-Match", ifNoneMatch, "ETag", serverETag)
	if strings.Contains(ifNoneMatch, ",") {
		for _, etag := range strings.Split(ifNoneMatch, ",") {
			if etag == serverETag {
				return true
			}
		}
		return false
	}

	ifNoneMatch, weakIfNoneMatch := parseETag(ifNoneMatch)
	serverETag, weakServerETag := parseETag(serverETag)
	slog.Info("ETag comparison 2", "If-None-Match", ifNoneMatch, "ETag", serverETag)
	if ifNoneMatch == serverETag {
		return true
	}
	if weakIfNoneMatch && weakServerETag {
		return true
	}
	return false
}
