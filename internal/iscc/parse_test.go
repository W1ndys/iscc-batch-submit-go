package iscc

import "testing"

func TestParseNonceFindsInputAndScriptForms(t *testing.T) {
	tests := []struct {
		name string
		html string
		want string
	}{
		{
			name: "input nonce",
			html: `<html><input name="nonce" value="abc123"></html>`,
			want: "abc123",
		},
		{
			name: "csrf input",
			html: `<html><input name="csrf_nonce" value="csrf123"></html>`,
			want: "csrf123",
		},
		{
			name: "script nonce",
			html: `<script>window.nonce = "script123"</script>`,
			want: "script123",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseNonce(tt.html)
			if err != nil {
				t.Fatalf("ParseNonce returned error: %v", err)
			}
			if got != tt.want {
				t.Fatalf("ParseNonce = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestParseChallengesSkipsSolvedAndSorts(t *testing.T) {
	html := `
		<a href="/chal/30">Pwn 30</a>
		<button data-id="15">Web 15</button>
		<div id="chal-50">Crypto 50</div>
	`

	got, err := ParseChallenges(html, map[int]struct{}{30: {}})
	if err != nil {
		t.Fatalf("ParseChallenges returned error: %v", err)
	}

	if len(got) != 2 {
		t.Fatalf("len(ParseChallenges) = %d, want 2", len(got))
	}
	if got[0].ID != 15 || got[0].Name != "Web 15" {
		t.Fatalf("got[0] = %+v, want id 15 name Web 15", got[0])
	}
	if got[1].ID != 50 || got[1].Name != "Crypto 50" {
		t.Fatalf("got[1] = %+v, want id 50 name Crypto 50", got[1])
	}
}

func TestParseTeamPath(t *testing.T) {
	got, ok := ParseTeamPath(`<a href="/team/abcdef">team</a>`)
	if !ok || got != "/team/abcdef" {
		t.Fatalf("ParseTeamPath = %q, %v", got, ok)
	}
}

func TestParseResultAndMessages(t *testing.T) {
	code, raw := ParseResult(200, []byte(`{"data":{"status":"correct","message":"ok"}}`))
	if code != "correct" {
		t.Fatalf("ParseResult code = %q, want correct", code)
	}
	if !IsSolvedResult(code, raw) {
		t.Fatalf("IsSolvedResult should be true for correct")
	}
	if msg := CodeToMessage("0", nil); msg != "❌ 错误" {
		t.Fatalf("CodeToMessage = %q", msg)
	}
}
