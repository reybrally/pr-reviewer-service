package http

import (
	stdhttp "net/http"
	"pr-reviewer-service/internal/service"
)

type Handler struct {
	teamService *service.TeamService
	userService *service.UserService
	prService   *service.PRService
}

func NewHandler(teamSvc *service.TeamService, userSvc *service.UserService, prSvc *service.PRService) *Handler {
	return &Handler{
		teamService: teamSvc,
		userService: userSvc,
		prService:   prSvc,
	}
}

func (h *Handler) RegisterRoutes(mux *stdhttp.ServeMux) {
	mux.HandleFunc("/health", h.handleHealth)

	mux.HandleFunc("/team/add", h.handleTeamAdd)
	mux.HandleFunc("/team/get", h.handleTeamGet)

	mux.HandleFunc("/users/setIsActive", h.handleUserSetIsActive)
	mux.HandleFunc("/users/getReview", h.handleUserGetReview)

	mux.HandleFunc("/pullRequest/create", h.handlePRCreate)
	mux.HandleFunc("/pullRequest/merge", h.handlePRMerge)
	mux.HandleFunc("/pullRequest/reassign", h.handlePRReassign)

	mux.HandleFunc("/stats/assignments", h.handleStatsAssignments)
	mux.HandleFunc("/team/deactivateMembers", h.handleTeamBulkDeactivate)
}
