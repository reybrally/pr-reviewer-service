package http

import (
	"encoding/json"
	"errors"
	stdhttp "net/http"
	"time"

	"pr-reviewer-service/internal/domain"
)

type prCreateRequest struct {
	PullRequestID   string `json:"pull_request_id"`
	PullRequestName string `json:"pull_request_name"`
	AuthorID        string `json:"author_id"`
}

type prMergeRequest struct {
	PullRequestID string `json:"pull_request_id"`
}

type prReassignRequest struct {
	PullRequestID string `json:"pull_request_id"`
	OldUserID     string `json:"old_user_id"`
}

type pullRequestDTO struct {
	PullRequestID     string   `json:"pull_request_id"`
	PullRequestName   string   `json:"pull_request_name"`
	AuthorID          string   `json:"author_id"`
	Status            string   `json:"status"`
	AssignedReviewers []string `json:"assigned_reviewers"`
	CreatedAt         string   `json:"createdAt,omitempty"`
	MergedAt          string   `json:"mergedAt,omitempty"`
}

type prCreateResponse struct {
	PR pullRequestDTO `json:"pr"`
}

type prMergeResponse struct {
	PR pullRequestDTO `json:"pr"`
}

type prReassignResponse struct {
	PR         pullRequestDTO `json:"pr"`
	ReplacedBy string         `json:"replaced_by"`
}

func (h *Handler) handlePRCreate(w stdhttp.ResponseWriter, r *stdhttp.Request) {
	if r.Method != stdhttp.MethodPost {
		w.Header().Set("Allow", stdhttp.MethodPost)
		writeError(w, stdhttp.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "method not allowed")
		return
	}

	defer func() {
		_ = r.Body.Close()
	}()
	var req prCreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, stdhttp.StatusBadRequest, "BAD_REQUEST", "invalid json body")
		return
	}

	if req.PullRequestID == "" || req.PullRequestName == "" || req.AuthorID == "" {
		writeError(w, stdhttp.StatusBadRequest, "BAD_REQUEST", "pull_request_id, pull_request_name and author_id are required")
		return
	}

	pr, err := h.prService.Create(
		r.Context(),
		domain.PullRequestID(req.PullRequestID),
		req.PullRequestName,
		domain.UserID(req.AuthorID),
	)
	if err != nil {
		switch {
		case errors.Is(err, domain.ErrPullRequestExists):
			writeError(w, stdhttp.StatusConflict, "PR_EXISTS", "PR id already exists")
		case errors.Is(err, domain.ErrNotFound):
			writeError(w, stdhttp.StatusNotFound, "NOT_FOUND", "resource not found")
		default:
			writeError(w, stdhttp.StatusInternalServerError, "INTERNAL", "internal error")
		}
		return
	}

	resp := prCreateResponse{
		PR: prToDTO(pr),
	}
	writeJSON(w, stdhttp.StatusCreated, resp)
}

func (h *Handler) handlePRMerge(w stdhttp.ResponseWriter, r *stdhttp.Request) {
	if r.Method != stdhttp.MethodPost {
		w.Header().Set("Allow", stdhttp.MethodPost)
		writeError(w, stdhttp.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "method not allowed")
		return
	}

	defer func() {
		_ = r.Body.Close()
	}()
	var req prMergeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, stdhttp.StatusBadRequest, "BAD_REQUEST", "invalid json body")
		return
	}

	if req.PullRequestID == "" {
		writeError(w, stdhttp.StatusBadRequest, "BAD_REQUEST", "pull_request_id is required")
		return
	}

	pr, err := h.prService.Merge(r.Context(), domain.PullRequestID(req.PullRequestID))
	if err != nil {
		switch {
		case errors.Is(err, domain.ErrNotFound):
			writeError(w, stdhttp.StatusNotFound, "NOT_FOUND", "resource not found")
		default:
			writeError(w, stdhttp.StatusInternalServerError, "INTERNAL", "internal error")
		}
		return
	}

	resp := prMergeResponse{
		PR: prToDTO(pr),
	}
	writeJSON(w, stdhttp.StatusOK, resp)
}

func (h *Handler) handlePRReassign(w stdhttp.ResponseWriter, r *stdhttp.Request) {
	if r.Method != stdhttp.MethodPost {
		w.Header().Set("Allow", stdhttp.MethodPost)
		writeError(w, stdhttp.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "method not allowed")
		return
	}

	defer func() {
		_ = r.Body.Close()
	}()
	var req prReassignRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, stdhttp.StatusBadRequest, "BAD_REQUEST", "invalid json body")
		return
	}

	if req.PullRequestID == "" || req.OldUserID == "" {
		writeError(w, stdhttp.StatusBadRequest, "BAD_REQUEST", "pull_request_id and old_user_id are required")
		return
	}

	pr, newReviewer, err := h.prService.Reassign(
		r.Context(),
		domain.PullRequestID(req.PullRequestID),
		domain.UserID(req.OldUserID),
	)
	if err != nil {
		switch {
		case errors.Is(err, domain.ErrNotFound):
			writeError(w, stdhttp.StatusNotFound, "NOT_FOUND", "resource not found")
		case errors.Is(err, domain.ErrPullRequestMerged):
			writeError(w, stdhttp.StatusConflict, "PR_MERGED", "cannot reassign on merged PR")
		case errors.Is(err, domain.ErrNotAssigned):
			writeError(w, stdhttp.StatusConflict, "NOT_ASSIGNED", "reviewer is not assigned to this PR")
		case errors.Is(err, domain.ErrNoCandidate):
			writeError(w, stdhttp.StatusConflict, "NO_CANDIDATE", "no active replacement candidate in team")
		default:
			writeError(w, stdhttp.StatusInternalServerError, "INTERNAL", "internal error")
		}
		return
	}

	resp := prReassignResponse{
		PR:         prToDTO(pr),
		ReplacedBy: string(newReviewer),
	}
	writeJSON(w, stdhttp.StatusOK, resp)
}

func prToDTO(pr domain.PullRequest) pullRequestDTO {
	dto := pullRequestDTO{
		PullRequestID:     string(pr.ID),
		PullRequestName:   pr.Name,
		AuthorID:          string(pr.AuthorID),
		Status:            string(pr.Status),
		AssignedReviewers: make([]string, 0, len(pr.AssignedReviewers)),
	}

	for _, r := range pr.AssignedReviewers {
		dto.AssignedReviewers = append(dto.AssignedReviewers, string(r))
	}

	if !pr.CreatedAt.IsZero() {
		dto.CreatedAt = pr.CreatedAt.UTC().Format(time.RFC3339)
	}
	if pr.MergedAt != nil {
		dto.MergedAt = pr.MergedAt.UTC().Format(time.RFC3339)
	}

	return dto
}
