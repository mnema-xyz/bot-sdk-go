package mnema

import (
	"context"
	"errors"
	"math/rand/v2"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"time"

	"github.com/gorilla/websocket"
)

// Gateway opcodes (doc/bots.md, Frames).
const (
	opDispatch       = 0
	opHeartbeat      = 1
	opIdentify       = 2
	opPresenceUpdate = 3
	opResume         = 6
	opReconnect      = 7
	opInvalidSession = 9
	opHello          = 10
	opHeartbeatACK   = 11
)

// Limits of frames from the bot.
const (
	maxFrameBytes   = 4096
	framesPerMinute = 120
)

// session is what a resume needs.
type session struct {
	id        string
	resumeURL string
	seq       int64
}

// MaxGatewayFrame is the largest frame the client reads from the gateway.
const MaxGatewayFrame = 4 << 20

func (c *Client) getSession() session {
	c.sessionMu.Lock()
	defer c.sessionMu.Unlock()
	return c.session
}

func (c *Client) updateSession(f func(*session)) {
	c.sessionMu.Lock()
	defer c.sessionMu.Unlock()
	f(&c.session)
}

// outcome is how one connection ended.
type outcome struct {
	code    int   // close code, 0 when the connection never opened
	fatal   error // stops Run
	healthy bool  // the connection reached READY or RESUMED
	err     error // why it ended, for the log
}

// Run connects to the gateway and delivers events to the handlers until ctx
// ends (it then returns nil) or the gateway closes with a fatal code (4004,
// 4013, 4014: it returns a *CloseError). Dropped connections are resumed
// with the session ID and the last sequence number; when the session cannot
// be resumed the client identifies again. Reconnects wait with exponential
// backoff and jitter between 1 and 60 seconds, longer after 4008. A REST
// answer of 401 or 403 to GET /gateway is returned as a *RESTError. Run
// waits for running handlers before it returns.
func (c *Client) Run(ctx context.Context) error {
	if !c.running.CompareAndSwap(false, true) {
		return ErrAlreadyRunning
	}
	defer c.running.Store(false)
	hctx, cancel := context.WithCancel(ctx)
	d := c.newDispatcher(hctx)
	defer func() {
		cancel()
		d.wait()
	}()
	attempt := 0
	for {
		out := c.connectOnce(ctx, d)
		if ctx.Err() != nil {
			return nil
		}
		if out.fatal != nil {
			return out.fatal
		}
		if out.healthy {
			attempt = 0
		}
		delay := c.backoff(attempt, out.code, rand.Float64)
		attempt++
		c.logger.Info("mnema: gateway connection ended, reconnecting", "code", out.code, "err", out.err, "in", delay)
		if sleep(ctx, delay) != nil {
			return nil
		}
	}
}

// backoff is attempt n's wait: between half and all of min * 2^n, capped at
// max; after 4008 at least rateLimitedBackoff.
func (c *Client) backoff(attempt, code int, random func() float64) time.Duration {
	base := c.maxBackoff
	if attempt < 30 {
		base = min(c.maxBackoff, c.minBackoff<<attempt)
	}
	d := base/2 + time.Duration(random()*float64(base/2))
	if code == CloseRateLimited {
		d = max(d, c.rateLimitedBackoff)
	}
	return d
}

// newSessionAfter reports close codes after which the session cannot be
// resumed, so the next connection identifies.
func newSessionAfter(code int) bool {
	switch code {
	case CloseSessionTakenOver, CloseNotAuthenticated, CloseAlreadyIdentified:
		return true
	}
	return false
}

func (c *Client) connectOnce(ctx context.Context, d *dispatcher) outcome {
	u, err := c.dialURL(ctx)
	if err != nil {
		var re *RESTError
		if errors.As(err, &re) && (re.Status == http.StatusUnauthorized || re.Status == http.StatusForbidden) {
			return outcome{fatal: err}
		}
		return outcome{err: err}
	}
	hdr := http.Header{"Authorization": {"Bot " + c.opts.Token}, "User-Agent": {userAgent()}}
	dialer := websocket.Dialer{Proxy: http.ProxyFromEnvironment, HandshakeTimeout: c.helloTimeout}
	ws, resp, err := dialer.DialContext(ctx, u, hdr)
	if resp != nil && resp.Body != nil {
		_ = resp.Body.Close()
	}
	if err != nil {
		c.updateSession(func(s *session) { s.resumeURL = "" })
		return outcome{err: err}
	}
	// No frame of the gateway comes near this (a message is at most 4000
	// characters plus embeds and components); a larger one ends the
	// connection instead of the bot's memory.
	ws.SetReadLimit(MaxGatewayFrame)
	cn := newConn(c, ws, u)
	out := cn.run(ctx, d)
	if newSessionAfter(out.code) {
		c.updateSession(func(s *session) { *s = session{} })
	}
	if IsFatalClose(out.code) {
		out.fatal = &CloseError{Code: out.code, Reason: cn.closeReason}
	}
	return out
}

// dialURL is the address of the next connection: the resume address of a
// live session, the configured GatewayURL, or what GET /gateway answers.
func (c *Client) dialURL(ctx context.Context) (string, error) {
	if s := c.getSession(); s.id != "" && s.resumeURL != "" {
		return s.resumeURL, nil
	}
	if c.opts.GatewayURL != "" {
		return c.opts.GatewayURL, nil
	}
	info, err := c.REST.Gateway(ctx)
	if err != nil {
		return "", err
	}
	if info.URL == "" {
		return wsURL(c.REST.baseURL) + BasePath + "/gateway", nil
	}
	return info.URL, nil
}

func wsURL(base string) string {
	if rest, ok := strings.CutPrefix(base, "https://"); ok {
		return "wss://" + rest
	}
	if rest, ok := strings.CutPrefix(base, "http://"); ok {
		return "ws://" + rest
	}
	return base
}

// sanitizeResumeURL is the address a resume dials. The upgrade carries the
// bot token, so a resume address never leaves the configured server: READY's
// resume_gateway_url is used only when it names the host and port of base,
// the address the session's connection dialled (or is relative), and then
// only its path and query, on base's scheme. Any other resume address, one
// that does not parse, or one whose path has a ".." segment gives base
// unchanged. An empty resume address stays empty (the next connection uses
// the normal address).
func sanitizeResumeURL(resume, base string) string {
	if resume == "" {
		return ""
	}
	b, err := url.Parse(base)
	if err != nil {
		return base
	}
	t, err := url.Parse(resume)
	if err != nil || !sameEndpoint(t, b) || slices.Contains(strings.Split(t.Path, "/"), "..") {
		return base
	}
	safe := url.URL{Scheme: b.Scheme, User: b.User, Host: b.Host, Path: b.Path, RawPath: b.RawPath, RawQuery: t.RawQuery}
	if t.Path != "" {
		safe.Path, safe.RawPath = t.Path, t.RawPath
	}
	return safe.String()
}

// sameEndpoint reports whether t is relative, or a ws or wss address of b's
// hostname and port (the scheme's default port when none is given).
func sameEndpoint(t, b *url.URL) bool {
	if t.Scheme == "" && t.Host == "" {
		return true
	}
	if t.Scheme != "ws" && t.Scheme != "wss" {
		return false
	}
	return strings.EqualFold(t.Hostname(), b.Hostname()) && portOf(t) == portOf(b)
}

func portOf(u *url.URL) string {
	if p := u.Port(); p != "" {
		return p
	}
	if u.Scheme == "wss" || u.Scheme == "https" {
		return "443"
	}
	return "80"
}
