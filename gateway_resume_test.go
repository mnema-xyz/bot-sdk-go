package mnema

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestAResumeDialsTheConfiguredServerWhateverREADYNames(t *testing.T) {
	var foreign atomic.Int32
	other := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		foreign.Add(1)
		http.NotFound(w, nil)
	}))
	t.Cleanup(other.Close)
	g := newFakeGateway(t, 60000)
	c := g.client(t, Options{Intents: IntentGuilds})
	runClient(t, c)
	fc := g.next(t)
	fc.expect(t, opIdentify)
	resume := "ws://" + strings.TrimPrefix(other.URL, "http://") + BasePath + "/gateway?resume=1"
	fc.dispatch(t, 1, "READY", map[string]any{"v": 1, "session_id": "s-1", "resume_gateway_url": resume,
		"user": map[string]any{"id": "u-bot", "bot_id": "b-1", "username": "bot-helper", "bot": true}})
	time.Sleep(20 * time.Millisecond)
	fc.closeWith(t, CloseSessionTimedOut, "")

	fc2 := g.next(t)
	fc2.expect(t, opResume)
	if fc2.query != "resume=1" {
		t.Fatalf("resume query = %q, want the READY's resume=1", fc2.query)
	}
	if got := fc2.header.Get("Authorization"); got != "Bot "+testToken {
		t.Fatalf("Authorization = %q", got)
	}
	if n := foreign.Load(); n != 0 {
		t.Fatalf("the host named in READY got %d requests, want none", n)
	}
}

func TestSanitizeResumeURLKeepsOnlyPathAndQuery(t *testing.T) {
	const base = "ws://talk.example.org:8080/api/bot/v1/gateway"
	cases := []struct{ resume, want string }{
		{"", ""},
		{"wss://elsewhere.example/api/bot/v1/gateway?r=1", "ws://talk.example.org:8080/api/bot/v1/gateway?r=1"},
		{"ws://talk.example.org:9999/resume", "ws://talk.example.org:8080/resume"},
		{"http://user:pw@elsewhere.example/x?y=2#frag", "ws://talk.example.org:8080/x?y=2"},
		{"wss://elsewhere.example", "ws://talk.example.org:8080/api/bot/v1/gateway"},
		{"ws://talk.example.org:8080/../admin", base},
		{"ws://bad host/%zz", base},
		{"/relative?q=1", "ws://talk.example.org:8080/relative?q=1"},
	}
	for _, tc := range cases {
		if got := sanitizeResumeURL(tc.resume, base); got != tc.want {
			t.Errorf("sanitizeResumeURL(%q) = %q, want %q", tc.resume, got, tc.want)
		}
	}
}
