package iscc

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"strings"
)

func CookieJarToRecords(jar http.CookieJar, baseURL string) []CookieRecord {
	if jar == nil {
		return nil
	}
	u, err := url.Parse(baseURL)
	if err != nil {
		return nil
	}
	cookies := jar.Cookies(u)
	records := make([]CookieRecord, 0, len(cookies))
	for _, cookie := range cookies {
		records = append(records, CookieRecord{
			Name:   cookie.Name,
			Value:  cookie.Value,
			Domain: cookie.Domain,
			Path:   cookie.Path,
		})
	}
	return records
}

func RecordsToCookieJar(records []CookieRecord) (*cookiejar.Jar, error) {
	jar, err := cookiejar.New(nil)
	if err != nil {
		return nil, err
	}
	if len(records) == 0 {
		return jar, nil
	}

	u, err := url.Parse(DefaultBaseURL)
	if err != nil {
		return nil, err
	}

	cookies := make([]*http.Cookie, 0, len(records))
	for _, record := range records {
		cookies = append(cookies, &http.Cookie{
			Name:   record.Name,
			Value:  record.Value,
			Domain: record.Domain,
			Path:   record.Path,
		})
	}
	jar.SetCookies(u, cookies)
	return jar, nil
}

func LoadCookieCache(path string) ([]CookieRecord, error) {
	if strings.TrimSpace(path) == "" {
		return nil, errors.New("cookie cache path is empty")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var cache CookieCache
	if err := json.Unmarshal(data, &cache); err != nil {
		var records []CookieRecord
		if err := json.Unmarshal(data, &records); err != nil {
			return nil, err
		}
		return records, nil
	}
	return cache.Cookies, nil
}

func SaveCookieCache(path string, baseURL string, jar http.CookieJar) error {
	cache := NewCookieCache(baseURL, CookieJarToRecords(jar, baseURL))
	data, err := json.MarshalIndent(cache, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}

func ParseCookieString(cookie string) []*http.Cookie {
	parts := strings.Split(cookie, ";")
	out := make([]*http.Cookie, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		pieces := strings.SplitN(part, "=", 2)
		if len(pieces) != 2 {
			continue
		}
		out = append(out, &http.Cookie{
			Name:  strings.TrimSpace(pieces[0]),
			Value: strings.TrimSpace(pieces[1]),
			Path:  "/",
		})
	}
	return out
}
