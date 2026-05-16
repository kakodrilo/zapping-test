package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"

	"zapping-test-service/internal/domain"
)

type streamUseCase interface {
	ListAll(ctx context.Context) ([]domain.Stream, error)
	GetPlaylist(ctx context.Context, streamID int) (domain.PlaylistSnapshot, error)
	ProxySegment(ctx context.Context, streamID int, file string, dst io.Writer) error
}

type StreamHandler struct {
	uc streamUseCase
}

func NewStreamHandler(uc streamUseCase) *StreamHandler {
	return &StreamHandler{uc: uc}
}

func (h *StreamHandler) List(w http.ResponseWriter, r *http.Request) {
	streams, err := h.uc.ListAll(r.Context())
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	type streamDTO struct {
		ID          int    `json:"id"`
		Title       string `json:"title"`
		Description string `json:"description"`
	}
	dtos := make([]streamDTO, len(streams))
	for i, s := range streams {
		dtos[i] = streamDTO{ID: s.ID, Title: s.Title, Description: s.Description}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(dtos)
}

func (h *StreamHandler) Playlist(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		http.Error(w, "invalid stream id", http.StatusBadRequest)
		return
	}

	snap, err := h.uc.GetPlaylist(r.Context(), id)
	if err != nil {
		http.Error(w, "stream not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/vnd.apple.mpegurl")
	w.Header().Set("Cache-Control", "no-cache")
	fmt.Fprintf(w, "#EXTM3U\n")
	fmt.Fprintf(w, "#EXT-X-VERSION:3\n")
	fmt.Fprintf(w, "#EXT-X-TARGETDURATION:%d\n", snap.TargetDuration)
	fmt.Fprintf(w, "#EXT-X-MEDIA-SEQUENCE:%d\n", snap.MediaSequence)
	for _, seg := range snap.Segments {
		fmt.Fprintf(w, "#EXTINF:%.6f,\n", seg.Duration)
		fmt.Fprintf(w, "/stream/%d/segments/%s\n", id, seg.Name)
	}
}

func (h *StreamHandler) Segment(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		http.Error(w, "invalid stream id", http.StatusBadRequest)
		return
	}
	file := r.PathValue("file")

	w.Header().Set("Content-Type", "video/mp2t")
	w.Header().Set("Cache-Control", "no-cache")
	if err := h.uc.ProxySegment(r.Context(), id, file, w); err != nil {
		http.Error(w, "segment unavailable", http.StatusBadGateway)
	}
}
