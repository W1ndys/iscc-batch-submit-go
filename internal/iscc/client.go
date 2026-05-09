package iscc

import (
	"bytes"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"
	"time"
)

type Config struct {
	BaseURL   string
	Cookie    string
	UseProxy  bool
	Proxy     string
	TrustEnv  bool
	Timeout   time.Duration
	UserAgent string
}

type Client struct {
	baseURL    string
	httpClient *http.Client
	jar        *cookiejar.Jar
	cookie     string
	userAgent  string
}

func NewClient(cfg Config) *Client {
	baseURL := cfg.BaseURL
	if baseURL == "" {
		baseURL = DefaultBaseURL
	}
	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = 20 * time.Second
	}
	jar, _ := cookiejar.New(nil)
	if cfg.Cookie != "" {
		u, _ := url.Parse(baseURL)
		jar.SetCookies(u, ParseCookieString(cfg.Cookie))
	}

	transport := &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   timeout,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		ForceAttemptHTTP2: false,
	}
	if !cfg.TrustEnv {
		transport.Proxy = nil
	}
	if cfg.UseProxy && cfg.Proxy != "" {
		if proxyURL, err := url.Parse(cfg.Proxy); err == nil {
			transport.Proxy = http.ProxyURL(proxyURL)
		}
	}

	client := &http.Client{
		Timeout:   timeout,
		Jar:       jar,
		Transport: transport,
	}

	return &Client{
		baseURL:    baseURL,
		httpClient: client,
		jar:        jar,
		cookie:     cfg.Cookie,
		userAgent:  defaultUserAgent(cfg.UserAgent),
	}
}

func defaultUserAgent(userAgent string) string {
	if userAgent != "" {
		return userAgent
	}
	return "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36"
}

func (c *Client) Login(username, password string, retries int, retryDelay time.Duration) error {
	attempts := retries + 1
	if attempts < 1 {
		attempts = 1
	}
	var lastErr error
	for i := 1; i <= attempts; i++ {
		req, err := http.NewRequest(http.MethodPost, c.baseURL+"/login", bytes.NewBufferString(url.Values{
			"name":     []string{username},
			"password": []string{password},
		}.Encode()))
		if err != nil {
			return err
		}
		req.Header.Set("User-Agent", c.userAgent)
		req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/apng,*/*;q=0.8")
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.Header.Set("Origin", c.baseURL)
		req.Header.Set("Referer", c.baseURL+"/login")

		resp, err := c.httpClient.Do(req)
		if err != nil {
			lastErr = err
			if i < attempts {
				time.Sleep(retryDelay)
				continue
			}
			return fmt.Errorf("登录请求失败：%w", err)
		}
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
		if resp.StatusCode >= 400 {
			return fmt.Errorf("登录失败：HTTP %d", resp.StatusCode)
		}
		return nil
	}
	return lastErr
}

func (c *Client) CookieJar() http.CookieJar {
	return c.jar
}

func (c *Client) SetCookies(records []CookieRecord) {
	u, err := url.Parse(c.baseURL)
	if err != nil {
		return
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
	c.jar.SetCookies(u, cookies)
}

func (c *Client) GetNonce() (string, error) {
	body, _, err := c.get("/challenges", map[string]string{
		"Referer": c.baseURL + "/",
	})
	if err != nil {
		return "", err
	}
	return ParseNonce(string(body))
}

func (c *Client) FetchChallenges() ([]Challenge, error) {
	body, _, err := c.get("/challenges", map[string]string{
		"Referer": c.baseURL + "/",
	})
	if err != nil {
		return nil, err
	}
	solvedIDs := map[int]struct{}{}
	if teamPath, ok := ParseTeamPath(string(body)); ok {
		if ids, err := c.fetchSolvedIDs(teamPath); err == nil {
			solvedIDs = ids
		}
	}
	return ParseChallenges(string(body), solvedIDs)
}

func (c *Client) FetchSolvedIDsFromChallenges() (map[int]struct{}, error) {
	body, _, err := c.get("/challenges", map[string]string{
		"Referer": c.baseURL + "/",
	})
	if err != nil {
		return nil, err
	}
	teamPath, ok := ParseTeamPath(string(body))
	if !ok {
		return map[int]struct{}{}, nil
	}
	return c.fetchSolvedIDs(teamPath)
}

func (c *Client) SubmitFlag(challengeID int, flag, nonce string) (*http.Response, error) {
	form := url.Values{}
	form.Set("key", flag)
	form.Set("nonce", nonce)
	req, err := http.NewRequest(http.MethodPost, fmt.Sprintf("%s/chal/%d", c.baseURL, challengeID), bytes.NewBufferString(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", c.userAgent)
	req.Header.Set("Accept", "*/*")
	req.Header.Set("Accept-Language", "zh-CN,zh;q=0.9,en;q=0.8")
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded; charset=UTF-8")
	req.Header.Set("X-Requested-With", "XMLHttpRequest")
	req.Header.Set("Origin", c.baseURL)
	req.Header.Set("Referer", c.baseURL+"/challenges")
	return c.httpClient.Do(req)
}

func (c *Client) fetchSolvedIDs(teamPath string) (map[int]struct{}, error) {
	teamID := strings.Trim(teamPath, "/")
	parts := strings.Split(teamID, "/")
	if len(parts) == 0 {
		return map[int]struct{}{}, nil
	}
	body, _, err := c.get("/solves/"+parts[len(parts)-1], map[string]string{
		"Accept":  "application/json",
		"Referer": c.baseURL + teamPath,
	})
	if err != nil {
		return nil, err
	}
	return ParseSolvedIDs(body), nil
}

func (c *Client) get(path string, headers map[string]string) ([]byte, int, error) {
	req, err := http.NewRequest(http.MethodGet, c.baseURL+path, nil)
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("User-Agent", c.userAgent)
	req.Header.Set("Connection", "close")
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	return body, resp.StatusCode, err
}
