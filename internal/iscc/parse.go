package iscc

import (
	"bytes"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"golang.org/x/net/html"
)

var (
	noncePatterns = []*regexp.Regexp{
		regexp.MustCompile(`(?i)name=["']nonce["'][^>]*value=["']([^"']+)["']`),
		regexp.MustCompile(`(?i)value=["']([^"']+)["'][^>]*name=["']nonce["']`),
		regexp.MustCompile(`(?i)nonce["']?\s*[:=]\s*["']([^"']+)["']`),
		regexp.MustCompile(`(?i)csrfNonce["']?\s*[:=]\s*["']([^"']+)["']`),
		regexp.MustCompile(`(?i)csrf_nonce["']?\s*[:=]\s*["']([^"']+)["']`),
	}
	challengeIDPattern = regexp.MustCompile(`(?:/chals?/|^chal-?)(\d+)|^(\d+)$`)
	teamPathPattern    = regexp.MustCompile(`['"](/team/[0-9a-fA-F]+)['"]`)
)

func ParseNonce(page string) (string, error) {
	doc, err := html.Parse(strings.NewReader(page))
	if err != nil {
		return "", err
	}

	var walk func(*html.Node) string
	walk = func(n *html.Node) string {
		if n.Type == html.ElementNode && n.Data == "input" {
			name := attr(n, "name")
			if (name == "nonce" || name == "csrf_nonce") && attr(n, "value") != "" {
				return strings.TrimSpace(attr(n, "value"))
			}
		}
		for child := n.FirstChild; child != nil; child = child.NextSibling {
			if value := walk(child); value != "" {
				return value
			}
		}
		return ""
	}

	if value := walk(doc); value != "" {
		return value, nil
	}

	for _, pattern := range noncePatterns {
		if match := pattern.FindStringSubmatch(page); len(match) > 1 {
			return strings.TrimSpace(match[1]), nil
		}
	}

	return "", fmt.Errorf("未能从 /challenges 页面解析到 nonce，可能 Cookie 失效或页面结构变化")
}

func ParseChallenges(page string, solvedIDs map[int]struct{}) ([]Challenge, error) {
	doc, err := html.Parse(strings.NewReader(page))
	if err != nil {
		return nil, err
	}

	found := map[int]string{}
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode && (n.Data == "a" || n.Data == "button" || n.Data == "div") {
			for _, candidate := range challengeCandidates(n) {
				id, ok := parseChallengeID(candidate)
				if !ok {
					continue
				}
				if _, solved := solvedIDs[id]; solved {
					continue
				}
				if _, exists := found[id]; !exists {
					name := strings.TrimSpace(nodeText(n))
					if name == "" {
						name = strconv.Itoa(id)
					}
					found[id] = name
				}
			}
		}

		for child := n.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(doc)

	ids := make([]int, 0, len(found))
	for id := range found {
		ids = append(ids, id)
	}
	sort.Ints(ids)

	challenges := make([]Challenge, 0, len(ids))
	for _, id := range ids {
		challenges = append(challenges, Challenge{ID: id, Name: found[id]})
	}
	return challenges, nil
}

func ParseTeamPath(page string) (string, bool) {
	doc, err := html.Parse(strings.NewReader(page))
	if err == nil {
		var walk func(*html.Node) (string, bool)
		walk = func(n *html.Node) (string, bool) {
			if n.Type == html.ElementNode && n.Data == "a" {
				href := attr(n, "href")
				if regexp.MustCompile(`^/team/[^/]+$`).MatchString(href) {
					return href, true
				}
			}
			for child := n.FirstChild; child != nil; child = child.NextSibling {
				if value, ok := walk(child); ok {
					return value, true
				}
			}
			return "", false
		}
		if value, ok := walk(doc); ok {
			return value, true
		}
	}

	if match := teamPathPattern.FindStringSubmatch(page); len(match) > 1 {
		return match[1], true
	}
	return "", false
}

func ParseResult(statusCode int, body []byte) (string, any) {
	text := strings.TrimSpace(string(body))
	if regexp.MustCompile(`^-?\d+$`).MatchString(text) {
		return text, text
	}

	var data any
	if err := json.Unmarshal(body, &data); err == nil {
		if object, ok := data.(map[string]any); ok {
			if code, ok := firstResultValue(object); ok {
				return strings.TrimSpace(fmt.Sprint(code)), data
			}
			if inner, ok := object["data"].(map[string]any); ok {
				if code, ok := firstResultValue(inner); ok {
					return strings.TrimSpace(fmt.Sprint(code)), data
				}
			}
			raw, _ := json.Marshal(data)
			return string(raw), data
		}
		return strings.TrimSpace(fmt.Sprint(data)), data
	}

	if text != "" {
		return text, text
	}
	return fmt.Sprintf("HTTP_%d", statusCode), text
}

func IsSolvedResult(code string, raw any) bool {
	switch strings.ToLower(strings.TrimSpace(code)) {
	case "1", "2", "true", "correct", "already_solved", "solved":
		return true
	}

	if object, ok := raw.(map[string]any); ok {
		encoded, _ := json.Marshal(object)
		rawText := strings.ToLower(string(encoded))
		for _, keyword := range []string{"correct", "already solved", "already_solved", "solved", "正确", "已经解决", "已解决"} {
			if strings.Contains(rawText, strings.ToLower(keyword)) {
				return true
			}
		}
	}
	return false
}

func CodeToMessage(code string, raw any) string {
	clean := strings.TrimSpace(code)
	if message, ok := ResultMessages[clean]; ok {
		return message
	}

	if object, ok := raw.(map[string]any); ok {
		if message, ok := firstMessageValue(object); ok {
			return strings.TrimSpace(fmt.Sprint(message))
		}
		if inner, ok := object["data"].(map[string]any); ok {
			if message, ok := firstMessageValue(inner); ok {
				return strings.TrimSpace(fmt.Sprint(message))
			}
		}
	}

	return "未知返回：" + clean
}

func ParseSolvedIDs(data []byte) map[int]struct{} {
	var payload any
	if err := json.Unmarshal(data, &payload); err != nil {
		return map[int]struct{}{}
	}

	object, ok := payload.(map[string]any)
	if !ok {
		return map[int]struct{}{}
	}

	solves, ok := object["solves"].([]any)
	if !ok {
		return map[int]struct{}{}
	}

	ids := map[int]struct{}{}
	for _, item := range solves {
		solve, ok := item.(map[string]any)
		if !ok {
			continue
		}
		switch value := solve["chalid"].(type) {
		case float64:
			ids[int(value)] = struct{}{}
		case string:
			if id, err := strconv.Atoi(value); err == nil {
				ids[id] = struct{}{}
			}
		}
	}
	return ids
}

func attr(n *html.Node, key string) string {
	for _, attr := range n.Attr {
		if attr.Key == key {
			return attr.Val
		}
	}
	return ""
}

func challengeCandidates(n *html.Node) []string {
	keys := []string{"href", "data-id", "data-chalid", "chalid", "id"}
	candidates := make([]string, 0, len(keys))
	for _, key := range keys {
		if value := attr(n, key); value != "" {
			candidates = append(candidates, value)
		}
	}
	return candidates
}

func parseChallengeID(candidate string) (int, bool) {
	match := challengeIDPattern.FindStringSubmatch(candidate)
	if len(match) == 0 {
		return 0, false
	}
	value := match[1]
	if value == "" {
		value = match[2]
	}
	id, err := strconv.Atoi(value)
	return id, err == nil
}

func nodeText(n *html.Node) string {
	var buf bytes.Buffer
	var walk func(*html.Node)
	walk = func(node *html.Node) {
		if node.Type == html.TextNode {
			text := strings.TrimSpace(node.Data)
			if text != "" {
				if buf.Len() > 0 {
					buf.WriteByte(' ')
				}
				buf.WriteString(text)
			}
		}
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(n)
	return buf.String()
}

func firstResultValue(object map[string]any) (any, bool) {
	for _, key := range []string{"code", "status", "result", "success"} {
		if value, ok := object[key]; ok {
			return value, true
		}
	}
	return nil, false
}

func firstMessageValue(object map[string]any) (any, bool) {
	for _, key := range []string{"message", "msg", "detail"} {
		if value, ok := object[key]; ok {
			return value, true
		}
	}
	return nil, false
}
