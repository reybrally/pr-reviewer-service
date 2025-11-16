package http_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"pr-reviewer-service/internal/service"
	"sync"
	"testing"
	"time"

	domain "pr-reviewer-service/internal/domain"
	httphandler "pr-reviewer-service/internal/http"
)

type inMemoryTeamRepo struct {
	mu    sync.RWMutex
	teams map[domain.TeamName]struct{}
}

func newInMemoryTeamRepo() *inMemoryTeamRepo {
	return &inMemoryTeamRepo{
		teams: make(map[domain.TeamName]struct{}),
	}
}

type inMemoryPRRepo struct {
	mu  sync.RWMutex
	prs map[domain.PullRequestID]domain.PullRequest
}

func (r *inMemoryTeamRepo) CreateTeam(ctx context.Context, name domain.TeamName) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.teams[name]; ok {
		return domain.ErrTeamExists
	}
	r.teams[name] = struct{}{}
	return nil
}

func (r *inMemoryTeamRepo) GetTeam(ctx context.Context, name domain.TeamName) (domain.Team, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if _, ok := r.teams[name]; !ok {
		return domain.Team{}, domain.ErrNotFound
	}

	var t domain.Team
	return t, nil
}

func (r *inMemoryTeamRepo) TeamExists(ctx context.Context, name domain.TeamName) (bool, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	_, ok := r.teams[name]
	return ok, nil
}

type inMemoryUserRepo struct {
	mu    sync.RWMutex
	users map[domain.UserID]domain.User
}

func newInMemoryUserRepo() *inMemoryUserRepo {
	return &inMemoryUserRepo{
		users: make(map[domain.UserID]domain.User),
	}
}

func (r *inMemoryUserRepo) UpsertUsers(ctx context.Context, users []domain.User) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, u := range users {
		r.users[u.ID] = u
	}
	return nil
}

func (r *inMemoryUserRepo) GetByID(ctx context.Context, id domain.UserID) (domain.User, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	u, ok := r.users[id]
	if !ok {
		return domain.User{}, domain.ErrNotFound
	}
	return u, nil
}

func (r *inMemoryUserRepo) SetIsActive(ctx context.Context, id domain.UserID, isActive bool) (domain.User, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	u, ok := r.users[id]
	if !ok {
		return domain.User{}, domain.ErrNotFound
	}
	u.IsActive = isActive
	r.users[id] = u
	return u, nil
}

func (r *inMemoryUserRepo) ListActiveByTeam(ctx context.Context, teamName domain.TeamName) ([]domain.User, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	res := make([]domain.User, 0)
	for _, u := range r.users {
		if u.TeamName == teamName && u.IsActive {
			res = append(res, u)
		}
	}
	return res, nil
}

func newInMemoryPRRepo() *inMemoryPRRepo {
	return &inMemoryPRRepo{
		prs: make(map[domain.PullRequestID]domain.PullRequest),
	}
}

func (r *inMemoryPRRepo) Create(ctx context.Context, pr domain.PullRequest) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.prs[pr.ID]; exists {
		return domain.ErrPullRequestExists
	}
	r.prs[pr.ID] = pr
	return nil
}

func (r *inMemoryPRRepo) Exists(ctx context.Context, id domain.PullRequestID) (bool, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	_, ok := r.prs[id]
	return ok, nil
}

func (r *inMemoryPRRepo) Get(ctx context.Context, id domain.PullRequestID) (domain.PullRequest, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	pr, ok := r.prs[id]
	if !ok {
		return domain.PullRequest{}, domain.ErrNotFound
	}
	return pr, nil
}

func (r *inMemoryPRRepo) MarkMerged(ctx context.Context, id domain.PullRequestID, mergedAt time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	pr, ok := r.prs[id]
	if !ok {
		return domain.ErrNotFound
	}
	if pr.Status == domain.PRStatusMerged {
		return nil
	}
	pr.Status = domain.PRStatusMerged
	pr.MergedAt = &mergedAt
	r.prs[id] = pr
	return nil
}

func (r *inMemoryPRRepo) StatsAssignmentsByUser(
	ctx context.Context,
) (map[domain.UserID]int, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	res := make(map[domain.UserID]int)

	for _, pr := range r.prs {
		for _, rid := range pr.AssignedReviewers {
			res[rid]++
		}
	}

	return res, nil
}

func (r *inMemoryPRRepo) ReplaceReviewer(ctx context.Context, prID domain.PullRequestID, oldUserID, newUserID domain.UserID) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	pr, ok := r.prs[prID]
	if !ok {
		return domain.ErrNotFound
	}

	found := false
	for i, id := range pr.AssignedReviewers {
		if id == oldUserID {
			pr.AssignedReviewers[i] = newUserID
			found = true
			break
		}
	}
	if !found {
		return domain.ErrNotAssigned
	}

	r.prs[prID] = pr
	return nil
}

func (r *inMemoryPRRepo) ListByReviewer(ctx context.Context, reviewerID domain.UserID) ([]domain.PullRequestShort, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]domain.PullRequestShort, 0)
	for _, pr := range r.prs {
		for _, rid := range pr.AssignedReviewers {
			if rid == reviewerID {
				result = append(result, domain.PullRequestShort{
					ID:       pr.ID,
					Name:     pr.Name,
					AuthorID: pr.AuthorID,
					Status:   pr.Status,
				})
				break
			}
		}
	}
	return result, nil
}

type testEnv struct {
	server *httptest.Server
	client *http.Client
}

func newTestEnv(t *testing.T) *testEnv {
	t.Helper()

	teamRepo := newInMemoryTeamRepo()
	userRepo := newInMemoryUserRepo()
	prRepo := newInMemoryPRRepo()

	teamSvc := service.NewTeamService(teamRepo, userRepo)
	userSvc := service.NewUserService(userRepo)
	prSvc := service.NewPRService(userRepo, prRepo)

	h := httphandler.NewHandler(teamSvc, userSvc, prSvc)
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	return &testEnv{
		server: srv,
		client: srv.Client(),
	}
}

func (e *testEnv) postJSON(t *testing.T, path string, body any) *http.Response {
	t.Helper()
	data, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal body: %v", err)
	}
	req, err := http.NewRequest(http.MethodPost, e.server.URL+path, bytes.NewReader(data))
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := e.client.Do(req)
	if err != nil {
		t.Fatalf("do request: %v", err)
	}
	return resp
}

func (e *testEnv) get(t *testing.T, path string) *http.Response {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, e.server.URL+path, nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	resp, err := e.client.Do(req)
	if err != nil {
		t.Fatalf("do request: %v", err)
	}
	return resp
}

func decodeBody[T any](t *testing.T, resp *http.Response, v *T) {
	t.Helper()
	defer func() {
		_ = resp.Body.Close()
	}()
	if err := json.NewDecoder(resp.Body).Decode(v); err != nil {
		t.Fatalf("decode body: %v", err)
	}
}

type prResponse struct {
	PR struct {
		ID                string   `json:"pull_request_id"`
		Name              string   `json:"pull_request_name"`
		AuthorID          string   `json:"author_id"`
		Status            string   `json:"status"`
		AssignedReviewers []string `json:"assigned_reviewers"`
	} `json:"pr"`
}

type prMergeResponse = prResponse

type prReassignResponse struct {
	PR struct {
		ID                string   `json:"pull_request_id"`
		Name              string   `json:"pull_request_name"`
		AuthorID          string   `json:"author_id"`
		Status            string   `json:"status"`
		AssignedReviewers []string `json:"assigned_reviewers"`
	} `json:"pr"`
	ReplacedBy string `json:"replaced_by"`
}

type userGetReviewResponse struct {
	UserID       string `json:"user_id"`
	PullRequests []struct {
		ID       string `json:"pull_request_id"`
		Name     string `json:"pull_request_name"`
		AuthorID string `json:"author_id"`
		Status   string `json:"status"`
	} `json:"pull_requests"`
}

type errorResponse struct {
	Error struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

func TestCreateAndMergePRFlow(t *testing.T) {
	env := newTestEnv(t)

	teamReq := map[string]any{
		"team_name": "backend",
		"members": []map[string]any{
			{"user_id": "u1", "username": "Alice", "is_active": true},
			{"user_id": "u2", "username": "Bob", "is_active": true},
			{"user_id": "u3", "username": "Charlie", "is_active": true},
			{"user_id": "u4", "username": "Diana", "is_active": true},
		},
	}

	resp := env.postJSON(t, "/team/add", teamReq)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected status 201 on /team/add, got %d", resp.StatusCode)
	}
	_ = resp.Body.Close()

	prCreateReq := map[string]any{
		"pull_request_id":   "pr-1001",
		"pull_request_name": "Add search",
		"author_id":         "u1",
	}

	resp = env.postJSON(t, "/pullRequest/create", prCreateReq)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected status 201 on /pullRequest/create, got %d", resp.StatusCode)
	}

	var prResp prResponse
	decodeBody(t, resp, &prResp)

	if prResp.PR.ID != "pr-1001" {
		t.Fatalf("unexpected pull_request_id: %s", prResp.PR.ID)
	}
	if prResp.PR.Status != "OPEN" {
		t.Fatalf("expected status OPEN, got %s", prResp.PR.Status)
	}
	if len(prResp.PR.AssignedReviewers) == 0 || len(prResp.PR.AssignedReviewers) > 2 {
		t.Fatalf("expected 1 or 2 assigned reviewers, got %v", prResp.PR.AssignedReviewers)
	}
	for _, rID := range prResp.PR.AssignedReviewers {
		if rID == "u1" {
			t.Fatalf("author must not be assigned as reviewer")
		}
	}

	assigned := append([]string(nil), prResp.PR.AssignedReviewers...)

	reviewerID := assigned[0]
	resp = env.get(t, "/users/getReview?user_id="+reviewerID)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected status 200 on /users/getReview, got %d", resp.StatusCode)
	}
	var reviewResp userGetReviewResponse
	decodeBody(t, resp, &reviewResp)

	if reviewResp.UserID != reviewerID {
		t.Fatalf("expected user_id %s, got %s", reviewerID, reviewResp.UserID)
	}
	if len(reviewResp.PullRequests) != 1 || reviewResp.PullRequests[0].ID != "pr-1001" {
		t.Fatalf("expected exactly one PR pr-1001 in getReview, got %+v", reviewResp.PullRequests)
	}

	mergeReq := map[string]any{
		"pull_request_id": "pr-1001",
	}

	resp = env.postJSON(t, "/pullRequest/merge", mergeReq)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected status 200 on first merge, got %d", resp.StatusCode)
	}
	var mergeResp1 prMergeResponse
	decodeBody(t, resp, &mergeResp1)
	if mergeResp1.PR.Status != "MERGED" {
		t.Fatalf("expected status MERGED after first merge, got %s", mergeResp1.PR.Status)
	}

	resp = env.postJSON(t, "/pullRequest/merge", mergeReq)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected status 200 on second merge, got %d", resp.StatusCode)
	}
	var mergeResp2 prMergeResponse
	decodeBody(t, resp, &mergeResp2)
	if mergeResp2.PR.Status != "MERGED" {
		t.Fatalf("expected status MERGED after second merge, got %s", mergeResp2.PR.Status)
	}

	if len(mergeResp1.PR.AssignedReviewers) != len(mergeResp2.PR.AssignedReviewers) {
		t.Fatalf("expected reviewers to stay the same after repeated merge, got %v vs %v",
			mergeResp1.PR.AssignedReviewers, mergeResp2.PR.AssignedReviewers)
	}
}

func TestReassignReviewerAndValidation(t *testing.T) {
	env := newTestEnv(t)

	teamReq := map[string]any{
		"team_name": "backend",
		"members": []map[string]any{
			{"user_id": "u1", "username": "Alice", "is_active": true},
			{"user_id": "u2", "username": "Bob", "is_active": true},
			{"user_id": "u3", "username": "Charlie", "is_active": true},
			{"user_id": "u4", "username": "Diana", "is_active": true},
		},
	}

	resp := env.postJSON(t, "/team/add", teamReq)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected status 201 on /team/add, got %d", resp.StatusCode)
	}
	_ = resp.Body.Close()

	prCreateReq := map[string]any{
		"pull_request_id":   "pr-2001",
		"pull_request_name": "Refactor handlers",
		"author_id":         "u1",
	}

	resp = env.postJSON(t, "/pullRequest/create", prCreateReq)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected status 201 on /pullRequest/create, got %d", resp.StatusCode)
	}
	var prResp prResponse
	decodeBody(t, resp, &prResp)

	if len(prResp.PR.AssignedReviewers) == 0 {
		t.Fatalf("expected at least one assigned reviewer")
	}
	oldReviewer := prResp.PR.AssignedReviewers[0]

	reassignReq := map[string]any{
		"pull_request_id": "pr-2001",
		"old_user_id":     oldReviewer,
	}

	resp = env.postJSON(t, "/pullRequest/reassign", reassignReq)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected status 200 on /pullRequest/reassign, got %d", resp.StatusCode)
	}
	var reassignResp prReassignResponse
	decodeBody(t, resp, &reassignResp)

	if reassignResp.ReplacedBy == "" || reassignResp.ReplacedBy == oldReviewer {
		t.Fatalf("expected replaced_by to be a new reviewer, got %q", reassignResp.ReplacedBy)
	}
	for _, rid := range reassignResp.PR.AssignedReviewers {
		if rid == oldReviewer {
			t.Fatalf("old reviewer should not stay in assigned_reviewers after reassign")
		}
	}

	mergeReq := map[string]any{
		"pull_request_id": "pr-2001",
	}
	resp = env.postJSON(t, "/pullRequest/merge", mergeReq)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected status 200 on merge, got %d", resp.StatusCode)
	}
	_ = resp.Body.Close()

	resp = env.postJSON(t, "/pullRequest/reassign", reassignReq)
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("expected status 409 on reassign for MERGED PR, got %d", resp.StatusCode)
	}
	var errResp errorResponse
	decodeBody(t, resp, &errResp)
	if errResp.Error.Code != "PR_MERGED" {
		t.Fatalf("expected error code PR_MERGED, got %s", errResp.Error.Code)
	}
}
