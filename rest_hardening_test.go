package mnema

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

func TestARedirectIsAnErrorAndTheTokenStaysWithTheConfiguredServer(t *testing.T) {
	var reached, withAuth atomic.Int32
	// The target shares the host but not the port, which the standard
	// redirect policy treats as the same host and sends Authorization to.
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reached.Add(1)
		if r.Header.Get("Authorization") != "" {
			withAuth.Add(1)
		}
		writeJSON(w, 200, Self{ID: "u"})
	}))
	t.Cleanup(target.Close)
	r, _ := restServer(t, func(w http.ResponseWriter, req *http.Request, _ int) {
		http.Redirect(w, req, target.URL+req.URL.Path, http.StatusFound)
	})
	_, err := r.Me(context.Background())
	var re *RESTError
	if !errors.As(err, &re) || re.Status != http.StatusFound {
		t.Fatalf("Me = %v, want a *RESTError with status 302", err)
	}
	if reached.Load() != 0 || withAuth.Load() != 0 {
		t.Fatalf("the redirect target got %d requests (%d with Authorization), want none", reached.Load(), withAuth.Load())
	}
}

func TestAFailedInteractionRequestDoesNotNameTheToken(t *testing.T) {
	const token = "aW50ZXJhY3Rpb24tdG9rZW4tc2VjcmV0"
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := ln.Addr().String()
	_ = ln.Close() // nothing listens there now: the request fails to connect
	r, err := NewREST(Options{BaseURL: "http://" + addr, Token: testToken, MaxRetries: -1})
	if err != nil {
		t.Fatal(err)
	}
	i := decodeInteraction(t, `{"id":"i1","token":"`+token+`","type":2,"data":{"name":"roll"},"member":{"user":{"id":"u","username":"max"}}}`, r)
	err = i.Reply(context.Background(), Response{Content: "hi"})
	if err == nil {
		t.Fatal("Reply to a closed port succeeded")
	}
	if strings.Contains(err.Error(), token) {
		t.Fatalf("error names the interaction token: %v", err)
	}
	if !strings.Contains(err.Error(), "/interactions/i1/:token/callback") {
		t.Fatalf("error does not name the route: %v", err)
	}
}

func TestRedactPathReplacesOnlyTheTokenSegment(t *testing.T) {
	cases := map[string]string{
		"/interactions/i1/tok/callback":           "/interactions/i1/:token/callback",
		"/interactions/i1/tok/messages/@original": "/interactions/i1/:token/messages/@original",
		"/webhooks/app/tok":                       "/webhooks/app/:token",
		"/interactions/i1":                        "/interactions/i1",
		"/channels/c/messages":                    "/channels/c/messages",
	}
	for in, want := range cases {
		if got := redactPath(in); got != want {
			t.Errorf("redactPath(%q) = %q, want %q", in, got, want)
		}
	}
}
