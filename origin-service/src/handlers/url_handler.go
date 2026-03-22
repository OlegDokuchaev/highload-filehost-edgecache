package handlers

import (
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"context"

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
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		writeError(w, http.StatusBadRequest, "missing file")
		return
	}
	defer file.Close()

	userID, err := h.tokenVerifier.Verify(r.Context(), r.Header.Get("Authorization"))
	if err != nil {
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
			writeError(w, http.StatusRequestEntityTooLarge, "payload too large")
			return
		}
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
}

func (h *URLHandler) getMyFiles(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	userID, err := h.tokenVerifier.Verify(r.Context(), r.Header.Get("Authorization"))
	if err != nil {
		writeError(w, http.StatusUnauthorized, "token invalid")
		return
	}

	items, err := h.uploadService.ListByUserID(userID)
	if err != nil {
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
}

func (h *URLHandler) getFileByID(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	fileID := strings.TrimPrefix(r.URL.Path, "/files/")
	if fileID == "" {
		writeError(w, http.StatusNotFound, "not found")
		return
	}

	result, err := h.uploadService.DownloadByFileID(r.Context(), fileID)
	if err != nil {
		if errors.Is(err, service.ErrNotFound) {
			writeError(w, http.StatusNotFound, "not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	etag := buildETag(result.Bytes)
	ifNoneMatch := strings.TrimSpace(r.Header.Get("If-None-Match"))
	if ifNoneMatch != "" && ifNoneMatch == etag {
		w.Header().Set("ETag", etag)
		w.Header().Set("Cache-Control", h.cacheControlValue())
		w.WriteHeader(http.StatusNotModified)
		return
	}

	w.Header().Set("Content-Type", result.ContentType)
	w.Header().Set("Cache-Control", h.cacheControlValue())
	w.Header().Set("ETag", etag)
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(result.Bytes)
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

func buildETag(data []byte) string {
	sum := sha1.Sum(data)
	return `"` + hex.EncodeToString(sum[:]) + `"`
}
