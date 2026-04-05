package upload

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"hiring-challenge-backend/api/internal/auth"
	"hiring-challenge-backend/api/internal/hasura"

	"github.com/google/uuid"
)

type Handler struct {
	hasuraClient     *hasura.Client
	uploadDir        string
	thumbnailMaxSize int
}

func NewHandler(hasuraClient *hasura.Client, uploadDir string, thumbnailMaxSize int) *Handler {
	return &Handler{
		hasuraClient:     hasuraClient,
		uploadDir:        uploadDir,
		thumbnailMaxSize: thumbnailMaxSize,
	}
}

func (h *Handler) HandleUploadImage(w http.ResponseWriter, r *http.Request) {
	claims, err := auth.RequireUserClaims(r)
	if err != nil {
		http.Error(w, "access denied", http.StatusUnauthorized)
		return
	}

	if err := r.ParseMultipartForm(25 << 20); err != nil {
		http.Error(w, "invalid multipart form", http.StatusBadRequest)
		return
	}

	projectID, err := uuid.Parse(r.FormValue("projectId"))
	if err != nil {
		http.Error(w, "invalid projectId", http.StatusBadRequest)
		return
	}

	project, err := h.hasuraClient.GetProjectForUser(r.Context(), projectID, claims.UserID)
	if err != nil {
		if errors.Is(err, hasura.ErrNotFound) {
			http.Error(w, "project access denied", http.StatusForbidden)
			return
		}
		http.Error(w, "failed to validate project", http.StatusInternalServerError)
		return
	}

	var parentID *uuid.UUID
	parentIDRaw := strings.TrimSpace(r.FormValue("parentId"))
	if parentIDRaw != "" {
		parsedParent, err := uuid.Parse(parentIDRaw)
		if err != nil {
			http.Error(w, "invalid parentId", http.StatusBadRequest)
			return
		}
		parentID = &parsedParent
	}

	file, header, err := r.FormFile("image")
	if err != nil {
		http.Error(w, "missing image file", http.StatusBadRequest)
		return
	}
	defer file.Close()

	fileBytes, err := io.ReadAll(io.LimitReader(file, 20<<20))
	if err != nil {
		http.Error(w, "failed to read image", http.StatusBadRequest)
		return
	}

	mimeType := DetectMimeType(fileBytes)
	if !strings.HasPrefix(mimeType, "image/") {
		http.Error(w, "file must be an image", http.StatusBadRequest)
		return
	}

	thumbnailBytes, err := CreateThumbnail(fileBytes, h.thumbnailMaxSize)
	if err != nil {
		http.Error(w, "failed to create thumbnail", http.StatusBadRequest)
		return
	}

	name := strings.TrimSpace(r.FormValue("name"))
	if name == "" {
		name = header.Filename
	}
	name = sanitizeName(name)
	now := time.Now().UTC()

	objectID := uuid.NewString()
	projectKey := projectID.String()

	originalRelPath := filepath.ToSlash(filepath.Join(projectKey, objectID+"_orig"))
	originalAbsPath := filepath.Join(h.uploadDir, originalRelPath)
	thumbnailDataURL := fmt.Sprintf(
		"data:image/jpeg;base64,%s",
		base64.StdEncoding.EncodeToString(thumbnailBytes),
	)

	if err := os.MkdirAll(filepath.Dir(originalAbsPath), 0o755); err != nil {
		http.Error(w, "failed to prepare upload directory", http.StatusInternalServerError)
		return
	}

	if err := os.WriteFile(originalAbsPath, fileBytes, 0o644); err != nil {
		http.Error(w, "failed to store image", http.StatusInternalServerError)
		return
	}

	nodeID, err := h.hasuraClient.InsertImageNode(r.Context(), hasura.InsertImageNodeInput{
		ProjectID:    projectID,
		ParentID:     parentID,
		GroupID:      project.GroupID,
		CreatedBy:    claims.UserID,
		Name:         name,
		MimeType:     mimeType,
		SizeBytes:    int64(len(fileBytes)),
		StorageKey:   originalRelPath,
		ThumbnailKey: thumbnailDataURL,
		CreatedAt:    now,
	})
	if err != nil {
		http.Error(w, "failed to insert image metadata", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"id":           nodeID.String(),
		"projectId":    projectID.String(),
		"parentId":     parentID,
		"name":         name,
		"mimeType":     mimeType,
		"sizeBytes":    len(fileBytes),
		"storageKey":   originalRelPath,
		"thumbnailKey": thumbnailDataURL,
		"createdAt":    now.Format(time.RFC3339),
	})
}

func sanitizeName(value string) string {
	value = strings.TrimSpace(value)
	value = strings.ReplaceAll(value, "/", "_")
	value = strings.ReplaceAll(value, "\\", "_")
	value = strings.ReplaceAll(value, "..", "_")
	if value == "" {
		return fmt.Sprintf("image-%s", uuid.NewString())
	}
	return value
}
