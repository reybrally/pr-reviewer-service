package http

import (
	"encoding/json"
	"errors"
	stdhttp "net/http"

	"pr-reviewer-service/internal/domain"
)

type teamMemberDTO struct {
	UserID   string `json:"user_id"`
	Username string `json:"username"`
	IsActive bool   `json:"is_active"`
}

type teamDTO struct {
	TeamName string          `json:"team_name"`
	Members  []teamMemberDTO `json:"members"`
}

type teamAddResponse struct {
	Team teamDTO `json:"team"`
}

func (h *Handler) handleTeamAdd(w stdhttp.ResponseWriter, r *stdhttp.Request) {
	if r.Method != stdhttp.MethodPost {
		w.Header().Set("Allow", stdhttp.MethodPost)
		writeError(w, stdhttp.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "method not allowed")
		return
	}

	defer func() {
		_ = r.Body.Close()
	}()
	var req teamDTO
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, stdhttp.StatusBadRequest, "BAD_REQUEST", "invalid json body")
		return
	}

	if req.TeamName == "" {
		writeError(w, stdhttp.StatusBadRequest, "BAD_REQUEST", "team_name is required")
		return
	}

	members := make([]domain.User, 0, len(req.Members))
	for _, m := range req.Members {
		if m.UserID == "" {
			continue
		}
		members = append(members, domain.User{
			ID:       domain.UserID(m.UserID),
			Username: m.Username,
			IsActive: m.IsActive,
		})
	}

	team, err := h.teamService.AddTeam(r.Context(), domain.TeamName(req.TeamName), members)
	if err != nil {
		switch {
		case errors.Is(err, domain.ErrTeamExists):
			writeError(w, stdhttp.StatusBadRequest, "TEAM_EXISTS", "team_name already exists")
		default:
			writeError(w, stdhttp.StatusInternalServerError, "INTERNAL", "internal error")
		}
		return
	}

	resp := teamAddResponse{
		Team: teamToDTO(team),
	}

	writeJSON(w, stdhttp.StatusCreated, resp)
}

func (h *Handler) handleTeamGet(w stdhttp.ResponseWriter, r *stdhttp.Request) {
	if r.Method != stdhttp.MethodGet {
		w.Header().Set("Allow", stdhttp.MethodGet)
		writeError(w, stdhttp.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "method not allowed")
		return
	}

	teamName := r.URL.Query().Get("team_name")
	if teamName == "" {
		writeError(w, stdhttp.StatusBadRequest, "BAD_REQUEST", "team_name is required")
		return
	}

	team, err := h.teamService.GetTeam(r.Context(), domain.TeamName(teamName))
	if err != nil {
		switch {
		case errors.Is(err, domain.ErrNotFound):
			writeError(w, stdhttp.StatusNotFound, "NOT_FOUND", "resource not found")
		default:
			writeError(w, stdhttp.StatusInternalServerError, "INTERNAL", "internal error")
		}
		return
	}

	resp := teamToDTO(team)
	writeJSON(w, stdhttp.StatusOK, resp)
}

func teamToDTO(t domain.Team) teamDTO {
	members := make([]teamMemberDTO, 0, len(t.Members))
	for _, m := range t.Members {
		members = append(members, teamMemberDTO{
			UserID:   string(m.ID),
			Username: m.Username,
			IsActive: m.IsActive,
		})
	}
	return teamDTO{
		TeamName: string(t.Name),
		Members:  members,
	}
}

type teamBulkDeactivateRequest struct {
	TeamName string   `json:"team_name"`
	UserIDs  []string `json:"user_ids"`
}

type teamBulkDeactivateResponse struct {
	TeamName string   `json:"team_name"`
	UserIDs  []string `json:"user_ids"`
}

func (h *Handler) handleTeamBulkDeactivate(w stdhttp.ResponseWriter, r *stdhttp.Request) {
	if r.Method != stdhttp.MethodPost {
		w.Header().Set("Allow", stdhttp.MethodPost)
		writeError(w, stdhttp.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "method not allowed")
		return
	}

	defer func() {
		_ = r.Body.Close()
	}()
	var req teamBulkDeactivateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, stdhttp.StatusBadRequest, "BAD_REQUEST", "invalid json body")
		return
	}

	if req.TeamName == "" || len(req.UserIDs) == 0 {
		writeError(w, stdhttp.StatusBadRequest, "BAD_REQUEST", "team_name and user_ids are required")
		return
	}

	ids := make([]domain.UserID, 0, len(req.UserIDs))
	for _, id := range req.UserIDs {
		ids = append(ids, domain.UserID(id))
	}

	if err := h.prService.BulkDeactivateAndReassign(r.Context(), ids); err != nil {
		writeError(w, stdhttp.StatusInternalServerError, "INTERNAL", "internal error")
		return
	}

	resp := teamBulkDeactivateResponse(req)

	writeJSON(w, stdhttp.StatusOK, resp)
}
