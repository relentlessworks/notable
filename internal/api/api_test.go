package api

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"

	"github.com/relentlessworks/notable/internal/auth"
	"github.com/relentlessworks/notable/internal/store"
)

func setupTestServer(t *testing.T) *Server {
	tmpFile, err := os.CreateTemp("", "notable-test-*.json")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	tmpFile.Close()
	t.Cleanup(func() { os.Remove(tmpFile.Name()) })
	s, err := store.New(tmpFile.Name())
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	a := auth.New("test-secret")
	return NewServer(s, a)
}

func TestHelp(t *testing.T) {
	srv := setupTestServer(t)

	req := httptest.NewRequest("GET", "/help", nil)
	w := httptest.NewRecorder()
	srv.Router(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "Notable") {
		t.Error("help text should contain 'Notable'")
	}
}

func TestHealth(t *testing.T) {
	srv := setupTestServer(t)

	req := httptest.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()
	srv.Router(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestAuthFlow(t *testing.T) {
	srv := setupTestServer(t)

	// Request OTP
	form := url.Values{"email": {"agent@test.com"}, "workspace": {"ws_test"}}
	req := httptest.NewRequest("POST", "/auth/request", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	srv.Router(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "code=") {
		t.Fatalf("expected code in response, got: %s", w.Body.String())
	}

	// Extract code
	codeStr := w.Body.String()
	idx := strings.Index(codeStr, "code=")
	if idx == -1 {
		t.Fatal("could not find code in response")
	}
	code := strings.TrimSpace(codeStr[idx+5:])

	// Verify OTP
	form2 := url.Values{"email": {"agent@test.com"}, "code": {code}}
	req2 := httptest.NewRequest("POST", "/auth/verify", strings.NewReader(form2.Encode()))
	req2.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w2 := httptest.NewRecorder()
	srv.Router(w2, req2)

	if w2.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w2.Code, w2.Body.String())
	}
	if !strings.Contains(w2.Body.String(), "token=nb_") {
		t.Fatalf("expected token in response, got: %s", w2.Body.String())
	}
}

func TestCreateAndGetNote(t *testing.T) {
	srv := setupTestServer(t)
	token := getTestToken(t, srv)

	// Create note
	form := url.Values{"title": {"Meeting Notes"}, "body": {"Discussed API design"}, "tags": {"work,meeting"}}
	req := httptest.NewRequest("POST", "/api/notes", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	srv.Router(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "handle=note_") {
		t.Fatalf("expected handle in response, got: %s", w.Body.String())
	}

	// Extract handle
	body := w.Body.String()
	idx := strings.Index(body, "handle=")
	handle := strings.TrimSpace(body[idx+7:])
	if sp := strings.Index(handle, " "); sp != -1 {
		handle = handle[:sp]
	}

	// Get note
	req2 := httptest.NewRequest("GET", "/api/notes/"+handle, nil)
	req2.Header.Set("Authorization", "Bearer "+token)
	w2 := httptest.NewRecorder()
	srv.Router(w2, req2)

	if w2.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w2.Code, w2.Body.String())
	}
	if !strings.Contains(w2.Body.String(), "Meeting Notes") {
		t.Errorf("expected title in response, got: %s", w2.Body.String())
	}
	if !strings.Contains(w2.Body.String(), "Discussed API design") {
		t.Errorf("expected body in response, got: %s", w2.Body.String())
	}
}

func TestListNotes(t *testing.T) {
	srv := setupTestServer(t)
	token := getTestToken(t, srv)

	// Create a note
	form := url.Values{"title": {"Test Note"}, "body": {"Some content"}, "tags": {"test"}}
	req := httptest.NewRequest("POST", "/api/notes", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	srv.Router(w, req)

	// List notes
	req2 := httptest.NewRequest("GET", "/api/notes", nil)
	req2.Header.Set("Authorization", "Bearer "+token)
	w2 := httptest.NewRecorder()
	srv.Router(w2, req2)

	if w2.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w2.Code, w2.Body.String())
	}
	if !strings.Contains(w2.Body.String(), "handle=note_") {
		t.Errorf("expected note in list, got: %s", w2.Body.String())
	}
}

func TestListNotesByTag(t *testing.T) {
	srv := setupTestServer(t)
	token := getTestToken(t, srv)

	// Create note with tag "work"
	form := url.Values{"title": {"Work Note"}, "body": {"Work content"}, "tags": {"work"}}
	req := httptest.NewRequest("POST", "/api/notes", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	srv.Router(w, req)

	// Create note with tag "personal"
	form2 := url.Values{"title": {"Personal Note"}, "body": {"Personal content"}, "tags": {"personal"}}
	req2 := httptest.NewRequest("POST", "/api/notes", strings.NewReader(form2.Encode()))
	req2.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req2.Header.Set("Authorization", "Bearer "+token)
	w2 := httptest.NewRecorder()
	srv.Router(w2, req2)

	// List notes filtered by tag=work
	req3 := httptest.NewRequest("GET", "/api/notes?tag=work", nil)
	req3.Header.Set("Authorization", "Bearer "+token)
	w3 := httptest.NewRecorder()
	srv.Router(w3, req3)

	if w3.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w3.Code, w3.Body.String())
	}
	if !strings.Contains(w3.Body.String(), "Work Note") {
		t.Errorf("expected 'Work Note' in filtered list, got: %s", w3.Body.String())
	}
	if strings.Contains(w3.Body.String(), "Personal Note") {
		t.Errorf("should not contain 'Personal Note' when filtering by tag=work, got: %s", w3.Body.String())
	}
}

func TestUpdateNote(t *testing.T) {
	srv := setupTestServer(t)
	token := getTestToken(t, srv)

	// Create note
	form := url.Values{"title": {"Original Title"}, "body": {"Original body"}, "tags": {"draft"}}
	req := httptest.NewRequest("POST", "/api/notes", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	srv.Router(w, req)

	body := w.Body.String()
	idx := strings.Index(body, "handle=")
	handle := strings.TrimSpace(body[idx+7:])
	if sp := strings.Index(handle, " "); sp != -1 {
		handle = handle[:sp]
	}

	// Update note
	form2 := url.Values{"title": {"Updated Title"}, "body": {"Updated body"}}
	req2 := httptest.NewRequest("PATCH", "/api/notes/"+handle, strings.NewReader(form2.Encode()))
	req2.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req2.Header.Set("Authorization", "Bearer "+token)
	w2 := httptest.NewRecorder()
	srv.Router(w2, req2)

	if w2.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w2.Code, w2.Body.String())
	}
	if !strings.Contains(w2.Body.String(), "Updated Title") {
		t.Errorf("expected updated title, got: %s", w2.Body.String())
	}

	// Verify by getting the note
	req3 := httptest.NewRequest("GET", "/api/notes/"+handle, nil)
	req3.Header.Set("Authorization", "Bearer "+token)
	w3 := httptest.NewRecorder()
	srv.Router(w3, req3)

	if !strings.Contains(w3.Body.String(), "Updated Title") {
		t.Errorf("expected updated title in get, got: %s", w3.Body.String())
	}
	if !strings.Contains(w3.Body.String(), "Updated body") {
		t.Errorf("expected updated body in get, got: %s", w3.Body.String())
	}
}

func TestDeleteNote(t *testing.T) {
	srv := setupTestServer(t)
	token := getTestToken(t, srv)

	// Create note
	form := url.Values{"title": {"To Delete"}, "body": {"Delete me"}}
	req := httptest.NewRequest("POST", "/api/notes", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	srv.Router(w, req)

	body := w.Body.String()
	idx := strings.Index(body, "handle=")
	handle := strings.TrimSpace(body[idx+7:])
	if sp := strings.Index(handle, " "); sp != -1 {
		handle = handle[:sp]
	}

	// Delete note
	req2 := httptest.NewRequest("DELETE", "/api/notes/"+handle, nil)
	req2.Header.Set("Authorization", "Bearer "+token)
	w2 := httptest.NewRecorder()
	srv.Router(w2, req2)

	if w2.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w2.Code, w2.Body.String())
	}
	if !strings.Contains(w2.Body.String(), "deleted") {
		t.Errorf("expected 'deleted' in response, got: %s", w2.Body.String())
	}

	// Verify it's gone
	req3 := httptest.NewRequest("GET", "/api/notes/"+handle, nil)
	req3.Header.Set("Authorization", "Bearer "+token)
	w3 := httptest.NewRecorder()
	srv.Router(w3, req3)

	if w3.Code != http.StatusNotFound {
		t.Fatalf("expected 404 after delete, got %d", w3.Code)
	}
}

func TestSearchNotes(t *testing.T) {
	srv := setupTestServer(t)
	token := getTestToken(t, srv)

	// Create notes
	form := url.Values{"title": {"API Design"}, "body": {"Discussed REST vs GraphQL"}}
	req := httptest.NewRequest("POST", "/api/notes", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	srv.Router(w, req)

	form2 := url.Values{"title": {"Grocery List"}, "body": {"Milk and eggs"}}
	req2 := httptest.NewRequest("POST", "/api/notes", strings.NewReader(form2.Encode()))
	req2.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req2.Header.Set("Authorization", "Bearer "+token)
	w2 := httptest.NewRecorder()
	srv.Router(w2, req2)

	// Search for "API"
	req3 := httptest.NewRequest("GET", "/api/notes/search?q=API", nil)
	req3.Header.Set("Authorization", "Bearer "+token)
	w3 := httptest.NewRecorder()
	srv.Router(w3, req3)

	if w3.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w3.Code, w3.Body.String())
	}
	if !strings.Contains(w3.Body.String(), "API Design") {
		t.Errorf("expected 'API Design' in search results, got: %s", w3.Body.String())
	}
	if strings.Contains(w3.Body.String(), "Grocery List") {
		t.Errorf("should not contain 'Grocery List' when searching for 'API', got: %s", w3.Body.String())
	}
}

func TestMissingTitle(t *testing.T) {
	srv := setupTestServer(t)
	token := getTestToken(t, srv)

	form := url.Values{"body": {"No title"}}
	req := httptest.NewRequest("POST", "/api/notes", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	srv.Router(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "hint:") {
		t.Errorf("expected hint in error response, got: %s", w.Body.String())
	}
}

func TestNoAuth(t *testing.T) {
	srv := setupTestServer(t)

	req := httptest.NewRequest("GET", "/api/notes", nil)
	w := httptest.NewRecorder()
	srv.Router(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "hint:") {
		t.Errorf("expected hint in error response, got: %s", w.Body.String())
	}
}

func TestJSONFormat(t *testing.T) {
	srv := setupTestServer(t)
	token := getTestToken(t, srv)

	// Create note
	form := url.Values{"title": {"JSON Test"}, "body": {"Testing JSON output"}}
	req := httptest.NewRequest("POST", "/api/notes", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/json")
	w := httptest.NewRecorder()
	srv.Router(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Header().Get("Content-Type"), "application/json") {
		t.Errorf("expected JSON content type, got: %s", w.Header().Get("Content-Type"))
	}
	if !strings.Contains(w.Body.String(), "\"handle\"") {
		t.Errorf("expected JSON with handle field, got: %s", w.Body.String())
	}
}

func TestWorkspaceInfo(t *testing.T) {
	srv := setupTestServer(t)
	token := getTestToken(t, srv)

	// Create a note
	form := url.Values{"title": {"Test"}, "body": {"Content"}}
	req := httptest.NewRequest("POST", "/api/notes", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	srv.Router(w, req)

	// Get workspace info
	req2 := httptest.NewRequest("GET", "/api/workspace", nil)
	req2.Header.Set("Authorization", "Bearer "+token)
	w2 := httptest.NewRecorder()
	srv.Router(w2, req2)

	if w2.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w2.Code, w2.Body.String())
	}
	if !strings.Contains(w2.Body.String(), "notes=1") {
		t.Errorf("expected notes=1 in workspace info, got: %s", w2.Body.String())
	}
}

func getTestToken(t *testing.T, srv *Server) string {
	// Request OTP
	form := url.Values{"email": {"agent@test.com"}, "workspace": {"ws_test"}}
	req := httptest.NewRequest("POST", "/auth/request", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	srv.Router(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("auth/request failed: %d %s", w.Code, w.Body.String())
	}

	body := w.Body.String()
	idx := strings.Index(body, "code=")
	if idx == -1 {
		t.Fatalf("no code in response: %s", body)
	}
	code := strings.TrimSpace(body[idx+5:])

	// Verify OTP
	form2 := url.Values{"email": {"agent@test.com"}, "code": {code}}
	req2 := httptest.NewRequest("POST", "/auth/verify", strings.NewReader(form2.Encode()))
	req2.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w2 := httptest.NewRecorder()
	srv.Router(w2, req2)

	if w2.Code != http.StatusOK {
		t.Fatalf("auth/verify failed: %d %s", w2.Code, w2.Body.String())
	}

	body2 := w2.Body.String()
	idx2 := strings.Index(body2, "token=")
	if idx2 == -1 {
		t.Fatalf("no token in response: %s", body2)
	}
	token := strings.TrimSpace(body2[idx2+6:])
	if sp := strings.Index(token, " "); sp != -1 {
		token = token[:sp]
	}

	return token
}
