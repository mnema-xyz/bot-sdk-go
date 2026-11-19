package mnema

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestDownloadAttachmentStreamsTheFileWithItsHeaders(t *testing.T) {
	r, reqs := restServer(t, func(w http.ResponseWriter, _ *http.Request, _ int) {
		w.Header().Set("Content-Type", "text/plain")
		w.Header().Set("Content-Disposition", `attachment; filename*=UTF-8''r%C3%A9sum%C3%A9.txt`)
		_, _ = w.Write([]byte("hello"))
	})
	d, err := r.DownloadAttachment(context.Background(), "a/1", DownloadOptions{})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = d.Body.Close() }()
	b, err := io.ReadAll(d.Body)
	if err != nil || string(b) != "hello" {
		t.Fatalf("body = %q, %v", b, err)
	}
	if d.ContentType != "text/plain" || d.Filename != "résumé.txt" || d.Size != 5 {
		t.Fatalf("download = %+v", d)
	}
	got := reqs()[0]
	if got.method != "GET" || got.path != "/api/bot/v1/media/a%2F1" || got.header.Get("Authorization") != "Bot "+testToken {
		t.Fatalf("request = %s %s %v", got.method, got.path, got.header)
	}
}

func TestDownloadAttachmentRefusesAnAnnouncedSizeOverMaxBytes(t *testing.T) {
	r, _ := restServer(t, func(w http.ResponseWriter, _ *http.Request, _ int) {
		_, _ = w.Write([]byte(strings.Repeat("x", 100)))
	})
	_, err := r.DownloadAttachment(context.Background(), "a", DownloadOptions{MaxBytes: 10})
	var mb *MaxBytesError
	if !errors.As(err, &mb) || mb.MaxBytes != 10 || mb.Bytes != 100 {
		t.Fatalf("err = %v, want a MaxBytesError for 100 bytes", err)
	}
}

func TestDownloadAttachmentCutsAnUnannouncedSizeOffWhileReading(t *testing.T) {
	r, _ := restServer(t, func(w http.ResponseWriter, _ *http.Request, _ int) {
		f := w.(http.Flusher)
		for range 4 {
			_, _ = w.Write([]byte(strings.Repeat("x", 8)))
			f.Flush() // chunked: no Content-Length
		}
	})
	d, err := r.DownloadAttachment(context.Background(), "a", DownloadOptions{MaxBytes: 20})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = d.Body.Close() }()
	if d.Size != -1 {
		t.Fatalf("Size = %d, want -1 for a chunked answer", d.Size)
	}
	b, err := io.ReadAll(d.Body)
	var mb *MaxBytesError
	if !errors.As(err, &mb) || mb.MaxBytes != 20 {
		t.Fatalf("err = %v, want a MaxBytesError", err)
	}
	if len(b) > 20 {
		t.Fatalf("read %d bytes past the limit of 20", len(b))
	}
}

func TestDownloadAttachmentMapsA404ToErrNotFound(t *testing.T) {
	r, _ := restServer(t, func(w http.ResponseWriter, _ *http.Request, _ int) {
		writeJSON(w, 404, map[string]any{"message": "Unknown Attachment", "code": 0})
	})
	_, err := r.DownloadAttachment(context.Background(), "gone", DownloadOptions{})
	var re *RESTError
	if !errors.Is(err, ErrNotFound) || !errors.As(err, &re) || re.Status != 404 {
		t.Fatalf("err = %v, want ErrNotFound and a RESTError", err)
	}
}

func TestDownloadAttachmentDoesNotFollowRedirects(t *testing.T) {
	r, reqs := restServer(t, func(w http.ResponseWriter, req *http.Request, _ int) {
		http.Redirect(w, req, "/elsewhere", http.StatusFound)
	})
	_, err := r.DownloadAttachment(context.Background(), "a", DownloadOptions{})
	var re *RESTError
	if !errors.As(err, &re) || re.Status != http.StatusFound {
		t.Fatalf("err = %v, want a RESTError with 302", err)
	}
	if n := len(reqs()); n != 1 {
		t.Fatalf("%d requests, want 1 (the redirect not followed)", n)
	}
}

func TestDownloadAttachmentEndsAtItsTimeout(t *testing.T) {
	release := make(chan struct{})
	t.Cleanup(func() { close(release) })
	r, _ := restServer(t, func(_ http.ResponseWriter, req *http.Request, _ int) {
		select {
		case <-release:
		case <-req.Context().Done():
		}
	})
	start := time.Now()
	_, err := r.DownloadAttachment(context.Background(), "a", DownloadOptions{Timeout: 50 * time.Millisecond})
	if !errors.Is(err, context.DeadlineExceeded) || time.Since(start) > 5*time.Second {
		t.Fatalf("err = %v after %v, want the deadline", err, time.Since(start))
	}
}

func TestDownloadAttachmentWaitsOutA429(t *testing.T) {
	r, reqs := restServer(t, func(w http.ResponseWriter, _ *http.Request, n int) {
		if n == 1 {
			writeJSON(w, 429, map[string]any{"message": "slow down", "retry_after": 0.01})
			return
		}
		_, _ = w.Write([]byte("ok"))
	})
	d, err := r.DownloadAttachment(context.Background(), "a", DownloadOptions{})
	if err != nil {
		t.Fatal(err)
	}
	_ = d.Body.Close()
	if n := len(reqs()); n != 2 {
		t.Fatalf("%d requests, want 2", n)
	}
}

func TestOtherRoutesAnswerErrNotFoundForA404(t *testing.T) {
	r, _ := restServer(t, func(w http.ResponseWriter, _ *http.Request, _ int) {
		writeJSON(w, 404, map[string]any{"message": "Unknown Message", "code": CodeUnknownMessage})
	})
	_, err := r.Message(context.Background(), "c", "m")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
	if errors.Is(&RESTError{Status: 403}, ErrNotFound) {
		t.Fatal("a 403 matches ErrNotFound")
	}
}

func TestAttachmentDecodesTheDocumentedAliases(t *testing.T) {
	var a Attachment
	raw := `{"id":"a","url":"/api/bot/v1/media/a","filename":"x.png","content_type":"image/png","size":42,` +
		`"original_filename":"x.png","mime_type":"image/png","size_bytes":42,"is_deleted":false}`
	if err := json.Unmarshal([]byte(raw), &a); err != nil {
		t.Fatal(err)
	}
	if a.Filename != "x.png" || a.ContentType != "image/png" || a.Size != 42 || a.SizeBytes != 42 {
		t.Fatalf("attachment = %+v", a)
	}
}
