package db

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
)

type Handlers struct {
	manager *Manager
}

func NewHandlers(manager *Manager) *Handlers {
	return &Handlers{manager: manager}
}

type GetResponse struct {
	Key    string      `json:"key"`
	Value  interface{} `json:"value"`
	Exists bool        `json:"exists"`
}

func (h *Handlers) Get(w http.ResponseWriter, r *http.Request) {
	namespace := h.namespaceFromPath(r)
	key := chi.URLParam(r, "key")

	store, err := h.manager.GetStore(namespace)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	value, err := store.Get("", key)
	if err != nil {
		writeJSON(w, http.StatusNotFound, GetResponse{
			Key:    key,
			Value:  nil,
			Exists: false,
		})
		return
	}

	var jsonValue interface{}
	if err := json.Unmarshal(value, &jsonValue); err != nil {
		// If not JSON, return as string
		jsonValue = string(value)
	}

	writeJSON(w, http.StatusOK, GetResponse{
		Key:    key,
		Value:  jsonValue,
		Exists: true,
	})
}

type SetRequest struct {
	Value interface{} `json:"value"`
	TTL   int         `json:"ttl,omitempty"` // Future: TTL support
}

func (h *Handlers) Set(w http.ResponseWriter, r *http.Request) {
	namespace := h.namespaceFromPath(r)
	key := chi.URLParam(r, "key")

	var req SetRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	value, err := json.Marshal(req.Value)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid value"})
		return
	}

	store, err := h.manager.GetStore(namespace)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	if err := store.Set("", key, value); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *Handlers) Delete(w http.ResponseWriter, r *http.Request) {
	namespace := h.namespaceFromPath(r)
	key := chi.URLParam(r, "key")

	store, err := h.manager.GetStore(namespace)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	if err := store.Delete("", key); err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "key not found"})
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

type ListResponse struct {
	Keys  []string `json:"keys"`
	Count int      `json:"count"`
}

func (h *Handlers) List(w http.ResponseWriter, r *http.Request) {
	namespace := h.namespaceFromPath(r)
	prefix := r.URL.Query().Get("prefix")
	limit := 100 // Default limit

	store, err := h.manager.GetStore(namespace)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	keys, err := store.List("", prefix, limit)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, ListResponse{
		Keys:  keys,
		Count: len(keys),
	})
}

func (h *Handlers) namespaceFromPath(r *http.Request) string {
	// Extract namespace from path: /api/db/{namespace}/{key}
	namespace := chi.URLParam(r, "namespace")
	if namespace == "" {
		return "default/"
	}
	// Ensure namespace ends with /
	if !strings.HasSuffix(namespace, "/") {
		namespace += "/"
	}
	return namespace
}

func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

