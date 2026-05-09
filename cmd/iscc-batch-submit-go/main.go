package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"
	"github.com/w1ndys/iscc-batch-submit-go/internal/runner"
)

func main() {
	cfg := runner.DefaultConfig()

	root := &cobra.Command{
		Use:   "iscc-submit",
		Short: "ISCC 自动登录并批量尝试提交 flag",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
			defer stop()
			if err := runner.Run(ctx, cfg, os.Stdout); err != nil {
				return err
			}
			return nil
		},
	}

	root.Flags().StringVar(&cfg.Cookie, "cookie", "", "登录后的 Cookie 字符串")
	root.Flags().StringVar(&cfg.CookieFile, "cookie-file", "", "从文件读取 Cookie")
	root.Flags().StringVar(&cfg.Username, "username", "", "ISCC 登录用户名，也可用 ISCC_USERNAME")
	root.Flags().StringVar(&cfg.Password, "password", "", "ISCC 登录密码，也可用 ISCC_PASSWORD")
	root.Flags().StringVar(&cfg.Flag, "flag", "", "单独提交一个 flag；传入后会覆盖 flags-file 的批量模式")
	root.Flags().StringVar(&cfg.CookieCache, "cookie-cache", cfg.CookieCache, "Cookie 缓存文件")
	root.Flags().StringVar(&cfg.FlagsFile, "flags-file", cfg.FlagsFile, "flag 文本文件，一行一个 flag")
	root.Flags().IntSliceVar(&cfg.Only, "only", nil, "只提交指定题目 ID，例如：--only 15,30,33")
	root.Flags().IntSliceVar(&cfg.Exclude, "exclude", nil, "排除指定题目 ID，例如：--exclude 50")
	root.Flags().IntVar(&cfg.Workers, "workers", 0, "每轮并发数量。默认 0 表示并发提交本轮所有题目")
	root.Flags().IntVar(&cfg.MaxRounds, "max-rounds", 0, "最大重试轮数。默认 0 表示无限重试")
	root.Flags().DurationVar(&cfg.RoundDelay, "round-delay", cfg.RoundDelay, "每轮重试之间的等待时间")
	root.Flags().DurationVar(&cfg.ThrottleDelay, "throttle-delay", cfg.ThrottleDelay, "遇到返回码 3 时的额外等待时间")
	root.Flags().DurationVar(&cfg.Timeout, "timeout", cfg.Timeout, "单次 HTTP 请求超时时间")
	root.Flags().IntVar(&cfg.LoginRetries, "login-retries", cfg.LoginRetries, "登录网络错误重试次数")
	root.Flags().DurationVar(&cfg.RetryDelay, "retry-delay", cfg.RetryDelay, "登录重试间隔")
	root.Flags().StringVar(&cfg.Nonce, "nonce", "", "手动指定 nonce。一般不建议使用")
	root.Flags().BoolVar(&cfg.TrustEnv, "trust-env", false, "允许使用系统代理环境变量，如 HTTPS_PROXY/ALL_PROXY")
	root.Flags().BoolVar(&cfg.UseProxy, "use-proxy", false, "启用代理")
	root.Flags().StringVar(&cfg.Proxy, "proxy", "", "代理地址，例如：http://127.0.0.1:8080")
	root.Flags().StringVar(&cfg.BaseURL, "base-url", cfg.BaseURL, "ISCC 平台地址")

	if err := root.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
