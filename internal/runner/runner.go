package runner

import (
	"context"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/w1ndys/iscc-batch-submit-go/internal/iscc"
)

func Run(ctx context.Context, cfg Config, out io.Writer) error {
	if cfg.BaseURL == "" {
		cfg.BaseURL = iscc.DefaultBaseURL
	}
	if cfg.CookieCache == "" {
		cfg.CookieCache = iscc.DefaultCookieCache
	}
	if cfg.FlagsFile == "" {
		cfg.FlagsFile = iscc.DefaultFlagsFile
	}

	if cfg.CookieFile != "" && cfg.Cookie == "" {
		cookie, err := os.ReadFile(cfg.CookieFile)
		if err != nil {
			return fmt.Errorf("读取 Cookie 文件失败：%w", err)
		}
		cfg.Cookie = strings.TrimSpace(string(cookie))
	}

	client, err := ensureClient(cfg, out)
	if err != nil {
		return err
	}

	flags, err := pickFlags(cfg.Flag, cfg.FlagsFile)
	if err != nil {
		return err
	}
	challenges, err := client.FetchChallenges()
	if err != nil {
		return err
	}
	challenges = filterChallenges(challenges, cfg.Only, cfg.Exclude)

	attempts := planAttempts(flags, challenges)
	if len(attempts) == 0 {
		fmt.Fprintln(out, "[!] 没有需要提交的 flag 或未解题目")
		return nil
	}

	printPlan(out, flags, challenges, attempts, cfg)
	pending := attempts
	roundNo := 0
	cookieSnapshot := iscc.CookieJarToRecords(client.CookieJar(), cfg.BaseURL)

	for len(pending) > 0 {
		select {
		case <-ctx.Done():
			fmt.Fprintln(out)
			fmt.Fprintln(out, "[!] 用户中断")
			fmt.Fprintf(out, "[!] 剩余尝试数量：%d\n", len(pending))
			fmt.Fprintln(out)
			fmt.Fprintln(out, "[*] 提交流程结束")
			return nil
		default:
		}

		roundNo++
		if cfg.MaxRounds > 0 && roundNo > cfg.MaxRounds {
			fmt.Fprintln(out)
			fmt.Fprintf(out, "[!] 已达到最大重试轮数：%d\n", cfg.MaxRounds)
			fmt.Fprintf(out, "[!] 剩余尝试数量：%d\n", len(pending))
			break
		}

		fmt.Fprintln(out)
		fmt.Fprintln(out, strings.Repeat("=", 60))
		fmt.Fprintf(out, "[*] 第 %d 轮并发提交\n", roundNo)
		fmt.Fprintf(out, "[*] 当前待提交数量：%d\n", len(pending))
		fmt.Fprintln(out, strings.Repeat("=", 60))

		workers := len(pending)
		if cfg.Workers > 0 && cfg.Workers < workers {
			workers = cfg.Workers
		}
		results := runRound(ctx, pending, workers, cfg, cookieSnapshot)
		solvedIDs := map[int]struct{}{}
		for _, result := range results {
			if result.Solved {
				solvedIDs[result.ChallengeID] = struct{}{}
			}
		}

		next := pending[:0]
		for _, attempt := range pending {
			if _, solved := solvedIDs[attempt.ChallengeID]; !solved {
				next = append(next, attempt)
			}
		}
		pending = next
		printRoundResult(out, roundNo, results, pending)

		if len(pending) == 0 {
			fmt.Fprintln(out)
			fmt.Fprintln(out, "[+] 所有题目均已正确或已解决")
			break
		}

		fmt.Fprintln(out)
		fmt.Fprintf(out, "[*] 等待 %.1f 秒后进入下一轮...\n", cfg.RoundDelay.Seconds())
		select {
		case <-ctx.Done():
			continue
		case <-time.After(cfg.RoundDelay):
		}
	}

	fmt.Fprintln(out)
	fmt.Fprintln(out, "[*] 提交流程结束")
	return nil
}

func ensureClient(cfg Config, out io.Writer) (*iscc.Client, error) {
	client := iscc.NewClient(iscc.Config{
		BaseURL:  cfg.BaseURL,
		Cookie:   cfg.Cookie,
		UseProxy: cfg.UseProxy,
		Proxy:    cfg.Proxy,
		TrustEnv: cfg.TrustEnv,
		Timeout:  cfg.Timeout,
	})

	if cfg.Cookie == "" {
		if records, err := iscc.LoadCookieCache(cfg.CookieCache); err == nil && len(records) > 0 {
			client.SetCookies(records)
			if _, err := client.GetNonce(); err == nil {
				fmt.Fprintf(out, "[*] 已复用 Cookie 缓存：%s\n", cfg.CookieCache)
				return client, nil
			} else {
				fmt.Fprintf(out, "[!] Cookie 缓存不可用，准备重新登录：%v\n", err)
			}
		}
	}

	username := firstNonEmpty(cfg.Username, os.Getenv("ISCC_USERNAME"))
	password := firstNonEmpty(cfg.Password, os.Getenv("ISCC_PASSWORD"))
	if cfg.Cookie == "" && (username == "" || password == "") {
		return nil, fmt.Errorf("[!] 请提供 --username/--password、ISCC_USERNAME/ISCC_PASSWORD，或使用 --cookie/--cookie-file")
	}

	if cfg.Cookie == "" {
		fmt.Fprintf(out, "[*] 正在登录账号：%s\n", username)
		if err := client.Login(username, password, cfg.LoginRetries, cfg.RetryDelay); err != nil {
			return nil, err
		}
		if err := iscc.SaveCookieCache(cfg.CookieCache, cfg.BaseURL, client.CookieJar()); err != nil {
			return nil, err
		}
		fmt.Fprintf(out, "[*] Cookie 已缓存：%s\n", cfg.CookieCache)
	}

	return client, nil
}

func runRound(ctx context.Context, pending []iscc.Attempt, workers int, cfg Config, cookies []iscc.CookieRecord) []iscc.SubmitResult {
	jobs := make(chan iscc.Attempt)
	results := make(chan iscc.SubmitResult, len(pending))
	var wg sync.WaitGroup

	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for attempt := range jobs {
				select {
				case <-ctx.Done():
					return
				default:
				}
				results <- submitOne(attempt, cfg, cookies)
			}
		}()
	}

	for _, attempt := range pending {
		jobs <- attempt
	}
	close(jobs)
	wg.Wait()
	close(results)

	out := make([]iscc.SubmitResult, 0, len(pending))
	for result := range results {
		out = append(out, result)
	}
	return out
}

func submitOne(attempt iscc.Attempt, cfg Config, cookies []iscc.CookieRecord) iscc.SubmitResult {
	result := iscc.SubmitResult{
		ChallengeID: attempt.ChallengeID,
		Name:        attempt.Name,
		Flag:        attempt.Flag,
	}
	client := iscc.NewClient(iscc.Config{
		BaseURL:  cfg.BaseURL,
		Cookie:   cfg.Cookie,
		UseProxy: cfg.UseProxy,
		Proxy:    cfg.Proxy,
		TrustEnv: cfg.TrustEnv,
		Timeout:  cfg.Timeout,
	})
	if cfg.Cookie == "" && len(cookies) > 0 {
		client.SetCookies(cookies)
	}

	nonce := strings.TrimSpace(cfg.Nonce)
	if nonce == "" {
		value, err := client.GetNonce()
		if err != nil {
			result.Error = fmt.Sprintf("%#v", err)
			return result
		}
		nonce = value
	}

	resp, err := client.SubmitFlag(attempt.ChallengeID, attempt.Flag, nonce)
	if err != nil {
		result.Error = fmt.Sprintf("%#v", err)
		return result
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		result.Error = fmt.Sprintf("%#v", err)
		return result
	}
	result.HTTPStatus = resp.StatusCode
	code, raw := iscc.ParseResult(resp.StatusCode, body)
	result.Code = strings.TrimSpace(code)
	result.Raw = raw
	result.Message = iscc.CodeToMessage(result.Code, raw)
	result.Solved = iscc.IsSolvedResult(result.Code, raw)
	result.OK = result.Solved
	if result.Code == "3" && cfg.ThrottleDelay > 0 {
		time.Sleep(cfg.ThrottleDelay)
	}
	return result
}

func pickFlags(singleFlag, flagsFile string) ([]string, error) {
	if singleFlag != "" {
		return []string{singleFlag}, nil
	}
	data, err := os.ReadFile(flagsFile)
	if err != nil {
		return nil, err
	}
	flags := []string{}
	for _, line := range strings.Split(string(data), "\n") {
		flag := strings.TrimSpace(line)
		if flag == "" || strings.HasPrefix(flag, "#") {
			continue
		}
		flags = append(flags, flag)
	}
	return flags, nil
}

func planAttempts(flags []string, challenges []iscc.Challenge) []iscc.Attempt {
	attempts := make([]iscc.Attempt, 0, len(flags)*len(challenges))
	for _, flag := range flags {
		for _, challenge := range challenges {
			attempts = append(attempts, iscc.Attempt{
				ChallengeID: challenge.ID,
				Name:        challenge.Name,
				Flag:        flag,
			})
		}
	}
	return attempts
}

func filterChallenges(challenges []iscc.Challenge, only, exclude []int) []iscc.Challenge {
	onlySet := intSet(only)
	excludeSet := intSet(exclude)
	out := challenges[:0]
	for _, challenge := range challenges {
		if len(onlySet) > 0 {
			if _, ok := onlySet[challenge.ID]; !ok {
				continue
			}
		}
		if _, ok := excludeSet[challenge.ID]; ok {
			continue
		}
		out = append(out, challenge)
	}
	return out
}

func printPlan(out io.Writer, flags []string, challenges []iscc.Challenge, attempts []iscc.Attempt, cfg Config) {
	fmt.Fprintln(out, "[*] 当前未解题目：")
	for _, challenge := range challenges {
		fmt.Fprintf(out, "    - %d | %s\n", challenge.ID, challenge.Name)
	}
	fmt.Fprintln(out)
	fmt.Fprintf(out, "[*] Flag 数量：%d\n", len(flags))
	fmt.Fprintf(out, "[*] 题目数量：%d\n", len(challenges))
	fmt.Fprintf(out, "[*] 尝试数量：%d\n", len(attempts))
	if cfg.Workers <= 0 {
		fmt.Fprintln(out, "[*] 并发配置：每轮全部并发")
	} else {
		fmt.Fprintf(out, "[*] 并发配置：%d\n", cfg.Workers)
	}
	if cfg.MaxRounds <= 0 {
		fmt.Fprintln(out, "[*] 最大轮数：无限")
	} else {
		fmt.Fprintf(out, "[*] 最大轮数：%d\n", cfg.MaxRounds)
	}
	fmt.Fprintf(out, "[*] 轮间隔  ：%.1f 秒\n", cfg.RoundDelay.Seconds())
	fmt.Fprintln(out, strings.Repeat("=", 60))
}

func printRoundResult(out io.Writer, roundNo int, results []iscc.SubmitResult, pending []iscc.Attempt) {
	fmt.Fprintln(out)
	fmt.Fprintln(out, strings.Repeat("-", 60))
	fmt.Fprintf(out, "[*] 第 %d 轮结果\n", roundNo)
	fmt.Fprintln(out, strings.Repeat("-", 60))

	sort.Slice(results, func(i, j int) bool {
		return results[i].ChallengeID < results[j].ChallengeID
	})
	for _, result := range results {
		fmt.Fprintln(out)
		fmt.Fprintf(out, "[*] 题目 ID : %d\n", result.ChallengeID)
		fmt.Fprintf(out, "[*] 题目名  : %s\n", result.Name)
		fmt.Fprintf(out, "[*] Flag   : %s\n", result.Flag)
		if result.HTTPStatus != 0 {
			fmt.Fprintf(out, "[*] HTTP   : %d\n", result.HTTPStatus)
		}
		if result.Code != "" {
			fmt.Fprintf(out, "[*] 返回码 : %s\n", result.Code)
			fmt.Fprintf(out, "[*] 结果   : %s\n", result.Message)
		}
		if result.Solved {
			fmt.Fprintln(out, "[+] 状态   : 成功或已解决")
		} else {
			fmt.Fprintln(out, "[-] 状态   : 未成功，后续继续重试")
		}
		if result.Error != "" {
			fmt.Fprintf(out, "[!] 错误   : %s\n", result.Error)
		}
	}

	solvedIDs := []int{}
	for _, result := range results {
		if result.Solved {
			solvedIDs = append(solvedIDs, result.ChallengeID)
		}
	}
	sort.Ints(solvedIDs)
	pendingIDs := []int{}
	for _, attempt := range pending {
		if _, exists := intSet(pendingIDs)[attempt.ChallengeID]; !exists {
			pendingIDs = append(pendingIDs, attempt.ChallengeID)
		}
	}
	sort.Ints(pendingIDs)

	fmt.Fprintln(out)
	fmt.Fprintln(out, strings.Repeat("-", 60))
	if len(solvedIDs) == 0 {
		fmt.Fprintln(out, "[*] 本轮成功或已解决：无")
	} else {
		fmt.Fprintf(out, "[*] 本轮成功或已解决：%v\n", solvedIDs)
	}
	if len(pendingIDs) == 0 {
		fmt.Fprintln(out, "[*] 剩余待提交：无")
	} else {
		fmt.Fprintf(out, "[*] 剩余待提交：%v\n", pendingIDs)
	}
	fmt.Fprintln(out, strings.Repeat("-", 60))
}

func intSet(values []int) map[int]struct{} {
	out := map[int]struct{}{}
	for _, value := range values {
		out[value] = struct{}{}
	}
	return out
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
