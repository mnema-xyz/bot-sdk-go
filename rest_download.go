package mnema

import (
	"context"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"time"
)

// DownloadOptions bound an attachment download.
type DownloadOptions struct {
	// MaxBytes refuses a larger file: before the body when the server
	// announces a larger Content-Length, else while reading. 0 means no
	// limit.
	MaxBytes int64
	// Timeout ends the request, reading the body included. 0 leaves it to
	// the context and the HTTP client.
	Timeout time.Duration
}

// DownloadedAttachment is an attachment being downloaded. The caller must
// close Body.
type DownloadedAttachment struct {
	// ContentType is the server's Content-Type, application/octet-stream if
	// none was sent.
	ContentType string
	// Size is the announced Content-Length, -1 when unknown.
	Size int64
	// Filename is the name from Content-Disposition, empty when none.
	Filename string
	// Body streams the file; with MaxBytes set, a read past the limit fails
	// with a *MaxBytesError.
	Body io.ReadCloser
}

// MaxBytesError reports an attachment larger than DownloadOptions.MaxBytes.
type MaxBytesError struct {
	MaxBytes int64
	// Bytes is the announced or the read size so far, at least MaxBytes+1.
	Bytes int64
}

func (e *MaxBytesError) Error() string {
	return fmt.Sprintf("mnema: attachment exceeds the maximum size of %d bytes (got %d)", e.MaxBytes, e.Bytes)
}

// DownloadAttachment streams the attachment with this id
// (GET /media/{id}, doc/bots.md "Downloading attachments"). The server
// answers 404 for a file the bot cannot view, which gives an error matching
// ErrNotFound. Redirects are not followed, a 429 is waited out like any
// request, and opts bound the size and the time.
func (r *REST) DownloadAttachment(ctx context.Context, id string, opts DownloadOptions) (*DownloadedAttachment, error) {
	path := "/media/" + url.PathEscape(id)
	cancel := context.CancelFunc(func() {})
	if opts.Timeout > 0 {
		ctx, cancel = context.WithTimeout(ctx, opts.Timeout)
	}
	res, err := r.download(ctx, path)
	if err != nil {
		cancel()
		return nil, err
	}
	if opts.MaxBytes > 0 && res.ContentLength > opts.MaxBytes {
		_ = res.Body.Close()
		cancel()
		return nil, &MaxBytesError{MaxBytes: opts.MaxBytes, Bytes: res.ContentLength}
	}
	contentType := res.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	return &DownloadedAttachment{
		ContentType: contentType,
		Size:        res.ContentLength,
		Filename:    dispositionFilename(res.Header.Get("Content-Disposition")),
		Body:        &cappedBody{body: res.Body, max: opts.MaxBytes, cancel: cancel},
	}, nil
}

// download sends GET path without following redirects and returns a 2xx
// answer with its body open; other answers become errors.
func (r *REST) download(ctx context.Context, path string) (*http.Response, error) {
	hc := *r.http
	hc.CheckRedirect = refuseRedirect
	key := bucketKey(http.MethodGet, path)
	for attempt := 0; ; attempt++ {
		if err := r.limits.acquire(ctx, key); err != nil {
			return nil, err
		}
		hr, err := http.NewRequestWithContext(ctx, http.MethodGet, r.baseURL+BasePath+path, nil)
		if err != nil {
			return nil, fmt.Errorf("mnema: build request: %w", err)
		}
		hr.Header.Set("Authorization", "Bot "+r.token)
		hr.Header.Set("User-Agent", userAgent())
		res, err := hc.Do(hr)
		if err != nil {
			return nil, fmt.Errorf("mnema: GET %s: %w", path, err)
		}
		if res.StatusCode >= 200 && res.StatusCode <= 299 {
			return res, nil
		}
		body, _ := io.ReadAll(io.LimitReader(res.Body, maxResponseBytes))
		_ = res.Body.Close()
		if res.StatusCode != http.StatusTooManyRequests {
			return nil, parseError(res.StatusCode, body)
		}
		rl := parseRateLimit(res.Header, body)
		if rl.RetryAfter > MaxRetryAfter || attempt >= r.maxRetries {
			return nil, rl
		}
		r.limits.pause(key, rl.Global, rl.RetryAfter+retryJitter())
	}
}

func dispositionFilename(v string) string {
	if v == "" {
		return ""
	}
	_, params, err := mime.ParseMediaType(v)
	if err != nil {
		return ""
	}
	return params["filename"]
}

// cappedBody fails a read past max (when max > 0) and ends the download's
// timeout when closed.
type cappedBody struct {
	body   io.ReadCloser
	max    int64
	read   int64
	cancel context.CancelFunc
}

func (b *cappedBody) Read(p []byte) (int, error) {
	n, err := b.body.Read(p)
	b.read += int64(n)
	if b.max > 0 && b.read > b.max {
		over := b.read - b.max
		return n - int(min(over, int64(n))), &MaxBytesError{MaxBytes: b.max, Bytes: b.read}
	}
	return n, err
}

func (b *cappedBody) Close() error {
	err := b.body.Close()
	b.cancel()
	return err
}

// ErrNotFound matches (with errors.Is) a REST answer of 404: an unknown or
// inaccessible attachment, message, channel or member.
var ErrNotFound = errors.New("mnema: not found")

// Is makes errors.Is(err, ErrNotFound) true for a 404.
func (e *RESTError) Is(target error) bool {
	return target == ErrNotFound && e.Status == http.StatusNotFound
}
