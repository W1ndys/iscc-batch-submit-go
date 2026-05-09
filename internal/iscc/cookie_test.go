package iscc

import (
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"testing"
)

func TestCookieCacheRoundTrip(t *testing.T) {
	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatalf("cookiejar.New: %v", err)
	}
	u, _ := url.Parse(DefaultBaseURL)
	jar.SetCookies(u, []*http.Cookie{{Name: "session", Value: "abc", Path: "/"}})

	records := CookieJarToRecords(jar, DefaultBaseURL)
	restored, err := RecordsToCookieJar(records)
	if err != nil {
		t.Fatalf("RecordsToCookieJar: %v", err)
	}

	cookies := restored.Cookies(u)
	if len(cookies) != 1 || cookies[0].Name != "session" || cookies[0].Value != "abc" {
		t.Fatalf("restored cookies = %+v", cookies)
	}
}
