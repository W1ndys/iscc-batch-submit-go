package iscc

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestClientLoginFetchAndSubmit(t *testing.T) {
	var sawLogin bool
	var sawSubmit bool

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/login":
			sawLogin = true
			if err := r.ParseForm(); err != nil {
				t.Fatalf("ParseForm: %v", err)
			}
			if r.Form.Get("name") != "user" || r.Form.Get("password") != "pass" {
				t.Fatalf("login form = %v", r.Form)
			}
			http.SetCookie(w, &http.Cookie{Name: "session", Value: "abc", Path: "/"})
			_, _ = w.Write([]byte("登出"))
		case "/challenges":
			_, _ = w.Write([]byte(`
				<input name="nonce" value="nonce-1">
				<a href="/team/abc123">team</a>
				<a href="/chal/11">Web 11</a>
				<a href="/chal/22">Pwn 22</a>
			`))
		case "/solves/abc123":
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{"solves": []map[string]any{{"chalid": 11}}})
		case "/chal/22":
			sawSubmit = true
			if err := r.ParseForm(); err != nil {
				t.Fatalf("ParseForm: %v", err)
			}
			if r.Form.Get("key") != "flag{ok}" || r.Form.Get("nonce") != "nonce-1" {
				t.Fatalf("submit form = %v", r.Form)
			}
			_, _ = w.Write([]byte("1"))
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	client := NewClient(Config{BaseURL: server.URL, Timeout: time.Second})
	if err := client.Login("user", "pass", 0, time.Millisecond); err != nil {
		t.Fatalf("Login: %v", err)
	}

	challenges, err := client.FetchChallenges()
	if err != nil {
		t.Fatalf("FetchChallenges: %v", err)
	}
	if len(challenges) != 1 || challenges[0].ID != 22 {
		t.Fatalf("FetchChallenges = %+v, want only challenge 22", challenges)
	}

	nonce, err := client.GetNonce()
	if err != nil {
		t.Fatalf("GetNonce: %v", err)
	}
	resp, err := client.SubmitFlag(22, "flag{ok}", nonce)
	if err != nil {
		t.Fatalf("SubmitFlag: %v", err)
	}
	defer resp.Body.Close()

	if !sawLogin || !sawSubmit {
		t.Fatalf("sawLogin=%v sawSubmit=%v", sawLogin, sawSubmit)
	}
}
