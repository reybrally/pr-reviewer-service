package http

import (
	"encoding/json"
	"errors"
	stdhttp "net/http"

	"pr-reviewer-service/internal/domain"
)

type setIsActiveRequest struct {
	UserID   string `json:"user_id"`
	IsActive bool   `json:"is_active"`
}

type userDTO struct {
	UserID   string `json:"user_id"`
	Username string `json:"username"`
	TeamName string `json:"team_name"`
	IsActive bool   `json:"is_active"`
}

type setIsActiveResponse struct {
	User userDTO `json:"user"`
}

type pullRequestShortDTO struct {
	PullRequestID   string `json:"pull_request_id"`
	PullRequestName string `json:"pull_request_name"`
	AuthorID        string `json:"author_id"`
	Status          string `json:"status"`
}

type userGetReviewResponse struct {
	UserID       string                `json:"user_id"`
	PullRequests []pullRequestShortDTO `json:"pull_requests"`
}

func (h *Handler) handleUserSetIsActive(w stdhttp.ResponseWriter, r *stdhttp.Request) {
	if r.Method != stdhttp.MethodPost {
		w.Header().Set("Allow", stdhttp.MethodPost)
		writeError(w, stdhttp.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "method not allowed")
		return
	}

	defer func() {
		_ = r.Body.Close()
	}()
	var req setIsActiveRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, stdhttp.StatusBadRequest, "BAD_REQUEST", "invalid json body")
		return
	}

	if req.UserID == "" {
		writeError(w, stdhttp.StatusBadRequest, "BAD_REQUEST", "user_id is required")
		return
	}

	user, err := h.userService.SetIsActive(r.Context(), domain.UserID(req.UserID), req.IsActive)
	if err != nil {
		switch {
		case errors.Is(err, domain.ErrNotFound):
			writeError(w, stdhttp.StatusNotFound, "NOT_FOUND", "resource not found")
		default:
			writeError(w, stdhttp.StatusInternalServerError, "INTERNAL", "internal error")
		}
		return
	}

	resp := setIsActiveResponse{
		User: userToDTO(user),
	}
	writeJSON(w, stdhttp.StatusOK, resp)
}

func (h *Handler) handleUserGetReview(w stdhttp.ResponseWriter, r *stdhttp.Request) {
	if r.Method != stdhttp.MethodGet {
		w.Header().Set("Allow", stdhttp.MethodGet)
		writeError(w, stdhttp.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "method not allowed")
		return
	}

	userID := r.URL.Query().Get("user_id")
	if userID == "" {
		writeError(w, stdhttp.StatusBadRequest, "BAD_REQUEST", "user_id is required")
		return
	}

	prs, err := h.prService.ListByReviewer(r.Context(), domain.UserID(userID))
	if err != nil {
		writeError(w, stdhttp.StatusInternalServerError, "INTERNAL", "internal error")
		return
	}

	resp := userGetReviewResponse{
		UserID:       userID,
		PullRequests: prsShortToDTO(prs),
	}

	writeJSON(w, stdhttp.StatusOK, resp)
}

func userToDTO(u domain.User) userDTO {
	return userDTO{
		UserID:   string(u.ID),
		Username: u.Username,
		TeamName: string(u.TeamName),
		IsActive: u.IsActive,
	}
}

func prsShortToDTO(prs []domain.PullRequestShort) []pullRequestShortDTO {
	res := make([]pullRequestShortDTO, 0, len(prs))
	for _, pr := range prs {
		res = append(res, pullRequestShortDTO{
			PullRequestID:   string(pr.ID),
			PullRequestName: pr.Name,
			AuthorID:        string(pr.AuthorID),
			Status:          string(pr.Status),
		})
	}
	return res
}
