package iscc

import "time"

const (
	DefaultBaseURL     = "https://iscc.isclab.org.cn"
	DefaultCookieCache = ".iscc_cookies.json"
	DefaultFlagsFile   = "flags.txt"
)

type Track string

const (
	TrackRegular Track = "regular"
	TrackArena   Track = "arena"
)

var ResultMessages = map[string]string{
	"1":  "✅ 正确",
	"0":  "❌ 错误",
	"2":  "🔒 已解决过",
	"3":  "⚡ 速度太快",
	"4":  "题目未开放或无权限",
	"5":  "服务器错误或未知错误",
	"-1": "nonce 错误或登录状态失效",
}

type Challenge struct {
	ID     int
	Name   string
	Track  Track
	Solves int
}

type Attempt struct {
	ChallengeID int
	Name        string
	Flag        string
	Track       Track
}

type SubmitResult struct {
	ChallengeID int
	Name        string
	Flag        string
	OK          bool
	Solved      bool
	Code        string
	Message     string
	HTTPStatus  int
	Raw         any
	Error       string
}

type CookieRecord struct {
	Name   string `json:"name"`
	Value  string `json:"value"`
	Domain string `json:"domain"`
	Path   string `json:"path"`
}

type CookieCache struct {
	BaseURL string         `json:"base_url"`
	SavedAt int64          `json:"saved_at"`
	Cookies []CookieRecord `json:"cookies"`
}

func NewCookieCache(baseURL string, cookies []CookieRecord) CookieCache {
	return CookieCache{
		BaseURL: baseURL,
		SavedAt: time.Now().Unix(),
		Cookies: cookies,
	}
}
