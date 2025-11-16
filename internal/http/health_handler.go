package http

import (
	stdhttp "net/http"
	"time"
)

type healthResponse struct {
	Status string    `json:"status"`
	Time   time.Time `json:"time"`
}

func (h *Handler) handleHealth(w stdhttp.ResponseWriter, r *stdhttp.Request) {
	if r.Method != stdhttp.MethodGet {
		w.Header().Set("Allow", stdhttp.MethodGet)
		writeError(w, stdhttp.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "method not allowed")
		return
	}

	resp := healthResponse{
		Status: "ok",
		Time:   time.Now().UTC(),
	}

	writeJSON(w, stdhttp.StatusOK, resp)
}
