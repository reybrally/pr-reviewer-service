package http

import (
	stdhttp "net/http"
)

type userStatsDTO struct {
	UserID      string `json:"user_id"`
	ReviewCount int    `json:"review_count"`
}

type statsAssignmentsResponse struct {
	ByUser []userStatsDTO `json:"by_user"`
}

func (h *Handler) handleStatsAssignments(w stdhttp.ResponseWriter, r *stdhttp.Request) {
	if r.Method != stdhttp.MethodGet {
		w.Header().Set("Allow", stdhttp.MethodGet)
		writeError(w, stdhttp.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "method not allowed")
		return
	}

	stats, err := h.prService.StatsAssignmentsByUser(r.Context())
	if err != nil {
		writeError(w, stdhttp.StatusInternalServerError, "INTERNAL", "internal error")
		return
	}

	resp := statsAssignmentsResponse{
		ByUser: make([]userStatsDTO, 0, len(stats)),
	}

	for id, cnt := range stats {
		resp.ByUser = append(resp.ByUser, userStatsDTO{
			UserID:      string(id),
			ReviewCount: cnt,
		})
	}

	writeJSON(w, stdhttp.StatusOK, resp)
}
