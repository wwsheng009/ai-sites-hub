package newapi

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"aiclient/internal/adapter"
)

func TestDetectAcceptsTopLevelAndNestedStatus(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{
			name: "reference shape",
			body: `{"success":true,"message":"","version":"v1.0.0-rc.31","system_name":"StarBridge","start_time":1789724180}`,
		},
		{
			name: "wrapped shape",
			body: `{"success":true,"message":"","data":{"version":"v1.0.0","system_name":"New API"}}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/api/status" {
					http.NotFound(w, r)
					return
				}
				w.Header().Set("Content-Type", "application/json")
				_, _ = io.WriteString(w, tt.body)
			}))
			t.Cleanup(srv.Close)

			ad, err := New(srv.URL, "", nil)
			if err != nil {
				t.Fatalf("New() error = %v", err)
			}
			got, err := ad.Detect(context.Background(), srv.URL)
			if err != nil {
				t.Fatalf("Detect() error = %v", err)
			}
			if got.Type != adapter.TypeNewAPI || got.Score < 8 {
				t.Fatalf("Detect() = %#v, want new-api score >= 8", got)
			}
			if got.FinalURL != srv.URL {
				t.Fatalf("FinalURL = %q, want %q", got.FinalURL, srv.URL)
			}
		})
	}
}

func TestLoginTokenSendsBearerHeader(t *testing.T) {
	const token = "token-for-test"
	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/user/self" {
			http.NotFound(w, r)
			return
		}
		gotAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"success":true,"message":"","data":{"id":1,"username":"tester"}}`)
	}))
	t.Cleanup(srv.Close)

	ad, err := New(srv.URL, "", nil)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	st, err := ad.Login(context.Background(), adapter.Credentials{
		AuthMode:    "token",
		AccessToken: "  Bearer " + token + " ",
	})
	if err != nil {
		t.Fatalf("Login() error = %v", err)
	}
	if st.State != adapter.StateOK || st.AccessToken != token {
		t.Fatalf("Login() state = %#v, want successful access token", st)
	}
	if gotAuth != "Bearer "+token {
		t.Fatalf("Authorization = %q, want %q", gotAuth, "Bearer "+token)
	}
}

func TestLoginInvalidTokenIsTokenExpired(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = io.WriteString(w, `{"success":false,"message":"unauthorized"}`)
	}))
	t.Cleanup(srv.Close)

	ad, err := New(srv.URL, "", nil)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	st, err := ad.Login(context.Background(), adapter.Credentials{
		AuthMode:    "token",
		AccessToken: "expired-token",
	})
	if err == nil {
		t.Fatal("Login() error = nil, want unauthorized error")
	}
	if st.State != adapter.StateTokenExpired {
		t.Fatalf("state = %q, want %q", st.State, adapter.StateTokenExpired)
	}
}

func TestLoginPasswordReturnsAccessToken(t *testing.T) {
	var got loginReq
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/user/login" {
			http.NotFound(w, r)
			return
		}
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Errorf("decode login request: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"success":true,"message":"","data":{"access_token":"jwt-from-login","token_type":"Bearer","access_expires_at":1893456000}}`)
	}))
	t.Cleanup(srv.Close)

	ad, err := New(srv.URL, "", nil)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	st, err := ad.Login(context.Background(), adapter.Credentials{
		AuthMode: "username_password",
		Username: "user",
		Password: "password",
	})
	if err != nil {
		t.Fatalf("Login() error = %v", err)
	}
	if got.Username != "user" || got.Password != "password" {
		t.Fatalf("login request = %#v", got)
	}
	if st.State != adapter.StateOK || st.AccessToken != "jwt-from-login" {
		t.Fatalf("Login() state = %#v, want access token", st)
	}
	if st.ExpiresHint == nil || !st.ExpiresHint.Equal(time.Unix(1893456000, 0)) {
		t.Fatalf("ExpiresHint = %v, want login expiry", st.ExpiresHint)
	}
}

func TestVerifySendsCookiesWhenNoToken(t *testing.T) {
	var gotCookie string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotCookie = r.Header.Get("Cookie")
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"success":true,"message":""}`)
	}))
	t.Cleanup(srv.Close)

	ad, err := New(srv.URL, "", nil)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	err = ad.Verify(context.Background(), adapter.AuthCtx{Cookies: []*http.Cookie{
		{Name: "new_api_refresh", Value: "opaque"},
	}})
	if err != nil {
		t.Fatalf("Verify() error = %v", err)
	}
	if !strings.Contains(gotCookie, "new_api_refresh=opaque") {
		t.Fatalf("Cookie = %q, want refresh cookie", gotCookie)
	}
}

// TestLiveCareke is opt-in so normal unit-test runs stay offline.  It is useful
// when validating an adapter against a deployed new-api instance:
// CAREKE_BASE_URL=https://... CAREKE_TOKEN=... go test ./internal/adapter/newapi -run TestLiveCareke -v
func TestLiveCareke(t *testing.T) {
	baseURL := strings.TrimSpace(os.Getenv("CAREKE_BASE_URL"))
	token := strings.TrimSpace(os.Getenv("CAREKE_TOKEN"))
	if baseURL == "" || token == "" {
		t.Skip("set CAREKE_BASE_URL and CAREKE_TOKEN to run the live new-api check")
	}

	ad, err := New(baseURL, "", nil)
	if err != nil {
		t.Fatal(err)
	}
	detected, err := ad.Detect(context.Background(), baseURL)
	if err != nil {
		t.Fatal(err)
	}
	if detected.Type != adapter.TypeNewAPI || detected.Score < 8 {
		t.Fatalf("Detect() = %#v, want new-api score >= 8", detected)
	}
	st, err := ad.Login(context.Background(), adapter.Credentials{
		AuthMode:    "token",
		AccessToken: token,
	})
	if err != nil {
		t.Fatalf("Login() error = %v, state = %#v", err, st)
	}
	if st.State != adapter.StateOK {
		t.Fatalf("Login() state = %#v, want ok", st)
	}
}
