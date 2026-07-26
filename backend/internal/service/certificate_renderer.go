package service

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"time"
)

// certificateRenderer turns self-contained HTML into rendered PDF bytes.
// Gotenberg is the only implementation today; the interface exists so
// certificate/card generation can be unit-tested without a real Gotenberg.
type certificateRenderer interface {
	RenderHTML(ctx context.Context, html []byte) ([]byte, error)
}

// defaultGotenbergTimeout bounds a single render end-to-end. Production injects
// http.DefaultClient, which has no timeout of its own, so without this a stalled
// Gotenberg would hang every certificate and card render indefinitely.
const defaultGotenbergTimeout = 30 * time.Second

// gotenbergRenderer calls a Gotenberg sidecar's Chromium HTML-to-PDF route
// directly via net/http + mime/multipart (FR-10: no third-party client lib).
type gotenbergRenderer struct {
	url        string
	httpClient *http.Client
	// timeout bounds one RenderHTML call, applied as a context deadline so it
	// covers the response-body read too and holds whatever client is injected.
	timeout time.Duration
}

func newGotenbergRenderer(url string, httpClient *http.Client) *gotenbergRenderer {
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	return &gotenbergRenderer{url: url, httpClient: httpClient, timeout: defaultGotenbergTimeout}
}

func (r *gotenbergRenderer) RenderHTML(ctx context.Context, html []byte) ([]byte, error) {
	if r.timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, r.timeout)
		defer cancel()
	}

	var body bytes.Buffer
	w := multipart.NewWriter(&body)

	fw, err := w.CreateFormFile("files", "index.html")
	if err != nil {
		return nil, fmt.Errorf("gotenberg: create html part: %w", err)
	}
	if _, err := fw.Write(html); err != nil {
		return nil, fmt.Errorf("gotenberg: write html part: %w", err)
	}

	fields := map[string]string{
		"printBackground":   "true",
		"preferCssPageSize": "true",
		"marginTop":         "0",
		"marginBottom":      "0",
		"marginLeft":        "0",
		"marginRight":       "0",
	}
	for k, v := range fields {
		if err := w.WriteField(k, v); err != nil {
			return nil, fmt.Errorf("gotenberg: write field %s: %w", k, err)
		}
	}

	if err := w.Close(); err != nil {
		return nil, fmt.Errorf("gotenberg: close multipart writer: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, r.url+"/forms/chromium/convert/html", &body)
	if err != nil {
		return nil, fmt.Errorf("gotenberg: build request: %w", err)
	}
	req.Header.Set("Content-Type", w.FormDataContentType())

	resp, err := r.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("gotenberg: request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("gotenberg: read response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("gotenberg: non-2xx response (status=%d): %s", resp.StatusCode, respBody)
	}

	return respBody, nil
}
