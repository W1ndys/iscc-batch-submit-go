package runner

import (
	"time"

	"github.com/w1ndys/iscc-batch-submit-go/internal/iscc"
)

type Config struct {
	BaseURL       string
	Cookie        string
	CookieFile    string
	Username      string
	Password      string
	Flag          string
	CookieCache   string
	FlagsFile     string
	Only          []int
	Exclude       []int
	Workers       int
	MaxRounds     int
	RoundDelay    time.Duration
	ThrottleDelay time.Duration
	Timeout       time.Duration
	LoginRetries  int
	RetryDelay    time.Duration
	Nonce         string
	TrustEnv      bool
	UseProxy      bool
	Proxy         string
}

func DefaultConfig() Config {
	return Config{
		BaseURL:       iscc.DefaultBaseURL,
		CookieCache:   iscc.DefaultCookieCache,
		FlagsFile:     iscc.DefaultFlagsFile,
		RoundDelay:    3 * time.Second,
		ThrottleDelay: 5 * time.Second,
		Timeout:       20 * time.Second,
		LoginRetries:  2,
		RetryDelay:    2 * time.Second,
	}
}
