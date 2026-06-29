package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/relentlessworks/notable/internal/auth"
	"github.com/relentlessworks/notable/internal/models"
	"github.com/relentlessworks/notable/internal/store"
)

// Server is the API server.
type Server struct {
	store *store.Store
	auth  *auth.AuthService
}

// NewServer creates a new API server.
func NewServer(s *store.Store, a *auth.AuthService) *Server {
	return &Server{store: s, auth: a}
}

// Router is the main HTTP router.
func (s *Server) Router(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path

	// --- Public routes ---

	// Help / agent manual
	if path == "/help" || path == "/.well-known/agent.md" {
		s.handleHelp(w, r)
		return
	}

	// Health check
	if path == "/health" {
		s.handleHealth(w, r)
		return
	}

	// Auth: request OTP
	if path == "/auth/request" && r.Method == "POST" {
		s.handleRequestOTP(w, r)
		return
	}

	// Auth: verify OTP
	if path == "/auth/verify" && r.Method == "POST" {
		s.handleVerifyOTP(w, r)
		return
	}

	// --- Authenticated routes ---
	// All /api/* routes require a bearer token

	if strings.HasPrefix(path, "/api/") {
		s.handleAPI(w, r)
		return
	}

	// Root: if nothing matched, show help
	if path == "/" {
		s.handleHelp(w, r)
		return
	}

	s.errorResponse(w, r, http.StatusNotFound, "not found", "GET /help to see available endpoints")
}

// --- Helpers ---

func (s *Server) wantsJSON(r *http.Request) bool {
	accept := r.Header.Get("Accept")
	if strings.Contains(accept, "application/json") {
		return true
	}
	if q := r.URL.Query().Get("format"); q == "json" {
		return true
	}
	return false
}

func (s *Server) writeResponse(w http.ResponseWriter, r *http.Request, status int, text string, data interface{}) {
	if s.wantsJSON(r) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		json.NewEncoder(w).Encode(data)
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(status)
	fmt.Fprintln(w, text)
}

func (s *Server) errorResponse(w http.ResponseWriter, r *http.Request, status int, msg, hint string) {
	if s.wantsJSON(r) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		json.NewEncoder(w).Encode(map[string]string{"error": msg, "hint": hint})
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(status)
	fmt.Fprintf(w, "error: %s\nhint: %s\n", msg, hint)
}

func (s *Server) getBearerToken(r *http.Request) string {
	auth := r.Header.Get("Authorization")
	if strings.HasPrefix(auth, "Bearer ") {
		return strings.TrimPrefix(auth, "Bearer ")
	}
	return ""
}

func (s *Server) authenticate(r *http.Request) (string, error) {
	token := s.getBearerToken(r)
	if token == "" {
		return "", fmt.Errorf("missing bearer token")
	}
	t, err := s.store.GetToken(token)
	if err != nil {
		return "", fmt.Errorf("invalid or expired token")
	}
	if time.Now().After(t.ExpiresAt) {
		return "", fmt.Errorf("token expired")
	}
	return t.Workspace, nil
}

// parseTags splits a comma-separated tag string into a slice.
func parseTags(tagStr string) []string {
	if tagStr == "" {
		return nil
	}
	var tags []string
	for _, t := range strings.Split(tagStr, ",") {
		t = strings.TrimSpace(t)
		if t != "" {
			tags = append(tags, t)
		}
	}
	return tags
}

// formatNote returns a one-line plain text representation of a note.
func formatNote(n *models.Note) string {
	tags := strings.Join(n.Tags, ",")
	if tags == "" {
		tags = "-"
	}
	return fmt.Sprintf("handle=%s title=%s tags=%s updated=%s", n.Handle, n.Title, tags, n.UpdatedAt.Format("2006-01-02T15:04:05Z"))
}

// formatNoteFull returns a multi-line plain text representation with the body.
func formatNoteFull(n *models.Note) string {
	tags := strings.Join(n.Tags, ",")
	if tags == "" {
		tags = "-"
	}
	return fmt.Sprintf("handle=%s\ntitle=%s\ntags=%s\ncreated=%s\nupdated=%s\n---\n%s",
		n.Handle, n.Title, tags,
		n.CreatedAt.Format("2006-01-02T15:04:05Z"),
		n.UpdatedAt.Format("2006-01-02T15:04:05Z"),
		n.Body)
}

// --- Handlers ---

func (s *Server) handleHelp(w http.ResponseWriter, r *http.Request) {
	help := `Notable — Agentic-First Notes & Knowledge Base
=================================================

Notable is a notes and knowledge base service designed for AI agents.
The API is the product. No UI, no SDK. Plain text by default, JSON on demand.

AUTHENTICATION
--------------
1. POST /auth/request   body: email=<email>&workspace=<handle>
   -> Sends a 6-digit OTP code (returned in plain text for local dev).
2. POST /auth/verify     body: email=<email>&code=<code>
   -> Returns a long-lived bearer token. Use it in Authorization: Bearer <token>.

CREATE A NOTE
-------------
POST /api/notes          body: title=<title>&body=<content>&tags=<comma-separated>
   -> Returns: handle=note_a1b2c title=My Note tags=idea,work updated=...

LIST NOTES
----------
GET /api/notes           -> One note per line: handle=note_a1b2c title=My Note tags=idea,work updated=...
GET /api/notes?tag=idea  -> Filter by tag

GET A NOTE
----------
GET /api/notes/<handle>  -> Full note with body (multi-line, body after ---)

UPDATE A NOTE
-------------
PATCH /api/notes/<handle>  body: title=<new-title>&body=<new-body>&tags=<new-tags>
   -> Any field can be omitted to keep the existing value.

DELETE A NOTE
-------------
DELETE /api/notes/<handle>

SEARCH NOTES
------------
GET /api/notes/search?q=<query>  -> Notes whose title or body contains the query

WORKSPACE INFO
--------------
GET /api/workspace        -> handle=ws_demo name=ws_demo plan=free notes=42

FORMATS
-------
- Plain text (default): one labeled, grepable line per record (full note shows body after ---).
- JSON: add Accept: application/json or ?format=json to any request.

ERRORS
------
4xx responses include an "error" and a "hint" field to guide you.

EXAMPLES
--------
  curl -X POST http://localhost:8080/auth/request -d 'email=me@example.com&workspace=ws_demo'
  curl -X POST http://localhost:8080/auth/verify -d 'email=me@example.com&code=123456'
  curl -X POST http://localhost:8080/api/notes -H 'Authorization: Bearer nb_xxx' -d 'title=Meeting Notes&body=Discussed API design&tags=work,meeting'
  curl http://localhost:8080/api/notes -H 'Authorization: Bearer nb_xxx'
  curl http://localhost:8080/api/notes?tag=work -H 'Authorization: Bearer nb_xxx'
  curl http://localhost:8080/api/notes/note_a1b2c -H 'Authorization: Bearer nb_xxx'
  curl 'http://localhost:8080/api/notes/search?q=API' -H 'Authorization: Bearer nb_xxx'

STORAGE
-------
Data is persisted to a JSON file (default: notable.json). Zero external dependencies.
`
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, help)
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	s.writeResponse(w, r, http.StatusOK, "ok", map[string]string{"status": "ok"})
}

func (s *Server) handleRequestOTP(w http.ResponseWriter, r *http.Request) {
	email := r.FormValue("email")
	workspace := r.FormValue("workspace")

	if email == "" || workspace == "" {
		s.errorResponse(w, r, http.StatusBadRequest, "missing email or workspace",
			"POST with email=<your-email>&workspace=<handle> (e.g. ws_demo)")
		return
	}

	// Auto-create workspace if it doesn't exist
	exists, err := s.store.WorkspaceExists(workspace)
	if err != nil {
		s.errorResponse(w, r, http.StatusInternalServerError, "database error", "try again")
		return
	}
	if !exists {
		if err := s.store.CreateWorkspace(workspace, workspace); err != nil {
			s.errorResponse(w, r, http.StatusInternalServerError, "failed to create workspace", "try a different workspace handle")
			return
		}
	}

	code := s.auth.GenerateOTP()
	if err := s.store.SaveOTP(email, code, workspace, auth.OTPExpiry()); err != nil {
		s.errorResponse(w, r, http.StatusInternalServerError, "failed to save OTP", "try again")
		return
	}

	// In production, email this. In dev, return it directly.
	s.writeResponse(w, r, http.StatusOK,
		fmt.Sprintf("otp_sent=true email=%s code=%s", email, code),
		map[string]string{"status": "otp_sent", "email": email, "code": code, "hint": "use POST /auth/verify with this code to get a token"},
	)
}

func (s *Server) handleVerifyOTP(w http.ResponseWriter, r *http.Request) {
	email := r.FormValue("email")
	code := r.FormValue("code")

	if email == "" || code == "" {
		s.errorResponse(w, r, http.StatusBadRequest, "missing email or code",
			"POST with email=<your-email>&code=<6-digit-code>")
		return
	}

	workspace, expiresAt, err := s.store.GetOTP(email, code)
	if err != nil {
		s.errorResponse(w, r, http.StatusUnauthorized, "invalid OTP code",
			"request a new OTP via POST /auth/request")
		return
	}

	if time.Now().After(expiresAt) {
		s.store.DeleteOTP(email, code)
		s.errorResponse(w, r, http.StatusUnauthorized, "OTP expired",
			"request a new OTP via POST /auth/request")
		return
	}

	// Delete used OTP
	s.store.DeleteOTP(email, code)

	// Generate token
	token := s.auth.GenerateToken(workspace)
	if err := s.store.CreateToken(token, workspace, auth.TokenExpiry()); err != nil {
		s.errorResponse(w, r, http.StatusInternalServerError, "failed to create token", "try again")
		return
	}

	s.writeResponse(w, r, http.StatusOK,
		fmt.Sprintf("token=%s workspace=%s", token, workspace),
		map[string]string{"token": token, "workspace": workspace, "hint": "use this token in Authorization: Bearer header"},
	)
}

func (s *Server) handleAPI(w http.ResponseWriter, r *http.Request) {
	workspace, err := s.authenticate(r)
	if err != nil {
		s.errorResponse(w, r, http.StatusUnauthorized, err.Error(),
			"POST /auth/request with email and workspace, then POST /auth/verify with the code")
		return
	}

	path := strings.TrimPrefix(r.URL.Path, "/api")

	switch {
	case path == "/notes" && r.Method == "POST":
		s.handleCreateNote(w, r, workspace)
	case path == "/notes" && r.Method == "GET":
		s.handleListNotes(w, r, workspace)
	case path == "/notes/search" && r.Method == "GET":
		s.handleSearchNotes(w, r, workspace)
	case strings.HasPrefix(path, "/notes/") && r.Method == "GET":
		s.handleGetNote(w, r, workspace)
	case strings.HasPrefix(path, "/notes/") && r.Method == "PATCH":
		s.handleUpdateNote(w, r, workspace)
	case strings.HasPrefix(path, "/notes/") && r.Method == "DELETE":
		s.handleDeleteNote(w, r, workspace)
	case path == "/workspace" && r.Method == "GET":
		s.handleGetWorkspace(w, r, workspace)
	default:
		s.errorResponse(w, r, http.StatusNotFound, "endpoint not found",
			"GET /help to see available endpoints")
	}
}

func (s *Server) handleCreateNote(w http.ResponseWriter, r *http.Request, workspace string) {
	title := r.FormValue("title")
	body := r.FormValue("body")
	tags := parseTags(r.FormValue("tags"))

	if title == "" {
		s.errorResponse(w, r, http.StatusBadRequest, "missing title",
			"POST with title=<note-title>&body=<content>&tags=<comma-separated>")
		return
	}

	handle := auth.GenerateHandle("note")
	if err := s.store.CreateNote(handle, title, body, tags, workspace); err != nil {
		s.errorResponse(w, r, http.StatusInternalServerError, "failed to create note", "try again")
		return
	}

	note, _ := s.store.GetNote(handle)
	s.writeResponse(w, r, http.StatusCreated,
		formatNote(note),
		note,
	)
}

func (s *Server) handleListNotes(w http.ResponseWriter, r *http.Request, workspace string) {
	tag := r.URL.Query().Get("tag")

	notes, err := s.store.ListNotes(workspace, tag, 50)
	if err != nil {
		s.errorResponse(w, r, http.StatusInternalServerError, "database error", "try again")
		return
	}

	if s.wantsJSON(r) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		if notes == nil {
			notes = []*models.Note{}
		}
		json.NewEncoder(w).Encode(notes)
		return
	}

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	if len(notes) == 0 {
		if tag != "" {
			fmt.Fprintf(w, "no notes found with tag '%s'. POST /api/notes with title=<title>&body=<content>&tags=%s to create one.\n", tag, tag)
		} else {
			fmt.Fprintln(w, "no notes found. POST /api/notes with title=<title>&body=<content> to create one.")
		}
		return
	}
	for _, n := range notes {
		fmt.Fprintln(w, formatNote(n))
	}
}

func (s *Server) handleGetNote(w http.ResponseWriter, r *http.Request, workspace string) {
	handle := strings.TrimPrefix(r.URL.Path, "/api/notes/")
	if handle == "" {
		s.errorResponse(w, r, http.StatusBadRequest, "missing handle", "GET /api/notes/<handle>")
		return
	}

	note, err := s.store.GetNote(handle)
	if err != nil {
		s.errorResponse(w, r, http.StatusNotFound, "note not found",
			"GET /api/notes to list all notes")
		return
	}

	// Verify ownership
	if note.Workspace != workspace {
		s.errorResponse(w, r, http.StatusNotFound, "note not found",
			"this note belongs to a different workspace")
		return
	}

	s.writeResponse(w, r, http.StatusOK,
		formatNoteFull(note),
		note,
	)
}

func (s *Server) handleUpdateNote(w http.ResponseWriter, r *http.Request, workspace string) {
	handle := strings.TrimPrefix(r.URL.Path, "/api/notes/")
	if handle == "" {
		s.errorResponse(w, r, http.StatusBadRequest, "missing handle", "PATCH /api/notes/<handle>")
		return
	}

	title := r.FormValue("title")
	body := r.FormValue("body")
	tagsStr := r.FormValue("tags")
	var tags []string
	if tagsStr != "" || r.FormValue("tags") != "" {
		tags = parseTags(tagsStr)
	}

	// Check if at least one field is being updated
	if title == "" && body == "" && tags == nil && r.FormValue("tags") == "" {
		s.errorResponse(w, r, http.StatusBadRequest, "no fields to update",
			"PATCH with title=<new-title>&body=<new-body>&tags=<new-tags> (any field can be omitted)")
		return
	}

	if err := s.store.UpdateNote(handle, title, body, tags, workspace); err != nil {
		s.errorResponse(w, r, http.StatusNotFound, "note not found",
			"GET /api/notes to list all notes")
		return
	}

	note, _ := s.store.GetNote(handle)
	s.writeResponse(w, r, http.StatusOK,
		formatNote(note),
		note,
	)
}

func (s *Server) handleDeleteNote(w http.ResponseWriter, r *http.Request, workspace string) {
	handle := strings.TrimPrefix(r.URL.Path, "/api/notes/")
	if handle == "" {
		s.errorResponse(w, r, http.StatusBadRequest, "missing handle", "DELETE /api/notes/<handle>")
		return
	}

	if err := s.store.DeleteNote(handle, workspace); err != nil {
		s.errorResponse(w, r, http.StatusNotFound, "note not found",
			"GET /api/notes to list all notes")
		return
	}

	s.writeResponse(w, r, http.StatusOK,
		fmt.Sprintf("deleted handle=%s", handle),
		map[string]string{"status": "deleted", "handle": handle},
	)
}

func (s *Server) handleSearchNotes(w http.ResponseWriter, r *http.Request, workspace string) {
	query := r.URL.Query().Get("q")
	if query == "" {
		s.errorResponse(w, r, http.StatusBadRequest, "missing query parameter",
			"GET /api/notes/search?q=<search-term>")
		return
	}

	notes, err := s.store.SearchNotes(workspace, query, 50)
	if err != nil {
		s.errorResponse(w, r, http.StatusInternalServerError, "database error", "try again")
		return
	}

	if s.wantsJSON(r) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		if notes == nil {
			notes = []*models.Note{}
		}
		json.NewEncoder(w).Encode(notes)
		return
	}

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	if len(notes) == 0 {
		fmt.Fprintf(w, "no notes found matching '%s'.\n", query)
		return
	}
	for _, n := range notes {
		fmt.Fprintln(w, formatNote(n))
	}
}

func (s *Server) handleGetWorkspace(w http.ResponseWriter, r *http.Request, workspace string) {
	ws, err := s.store.GetWorkspace(workspace)
	if err != nil {
		s.errorResponse(w, r, http.StatusNotFound, "workspace not found", "create it via POST /auth/request")
		return
	}

	notes, _ := s.store.ListNotes(workspace, "", 1000)
	noteCount := len(notes)

	s.writeResponse(w, r, http.StatusOK,
		fmt.Sprintf("handle=%s name=%s plan=%s notes=%d", ws.Handle, ws.Name, ws.Plan, noteCount),
		map[string]string{"handle": ws.Handle, "name": ws.Name, "plan": ws.Plan, "notes": fmt.Sprintf("%d", noteCount)},
	)
}
