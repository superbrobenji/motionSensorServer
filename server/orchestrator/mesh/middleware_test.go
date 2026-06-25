package mesh

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAuthMiddleware_MissingKey(t *testing.T) {
	handler := AuthMiddleware("secret")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/nodes", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

func TestAuthMiddleware_WrongKey(t *testing.T) {
	handler := AuthMiddleware("secret")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/nodes", nil)
	req.Header.Set("Authorization", "Bearer wrong")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

func TestAuthMiddleware_ValidKey(t *testing.T) {
	handler := AuthMiddleware("secret")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/nodes", nil)
	req.Header.Set("Authorization", "Bearer secret")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestCORSMiddleware_AllowedOrigin(t *testing.T) {
	handler := CORSMiddleware([]string{"http://localhost:3000"})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/nodes", nil)
	req.Header.Set("Origin", "http://localhost:3000")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Header().Get("Access-Control-Allow-Origin") != "http://localhost:3000" {
		t.Errorf("expected CORS header, got %q", w.Header().Get("Access-Control-Allow-Origin"))
	}
}

func TestCORSMiddleware_BlockedOrigin(t *testing.T) {
	handler := CORSMiddleware([]string{"http://localhost:3000"})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/nodes", nil)
	req.Header.Set("Origin", "http://evil.example.com")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Header().Get("Access-Control-Allow-Origin") != "" {
		t.Errorf("should not set CORS header for blocked origin")
	}
}

func TestIsValidAdapterType(t *testing.T) {
	valid := []int32{-1, 0, 1, 2, 3}
	for _, v := range valid {
		if !isValidAdapterType(v) {
			t.Errorf("expected %d to be valid", v)
		}
	}
	invalid := []int32{-2, 4, 99, -100}
	for _, v := range invalid {
		if isValidAdapterType(v) {
			t.Errorf("expected %d to be invalid", v)
		}
	}
}
