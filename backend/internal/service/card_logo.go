package service

import (
	"bytes"
	"context"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"syscall"
	"time"
)

// The exam card carries two images with very different trust: the student's
// photo_url, which the student controls, and app_logo_url, which only a
// super_admin can set through System Config. They must not share a loader.
//
// photo_url is read from our own object storage BY KEY and never triggers an
// outbound request (see avatarKeyFromStored). app_logo_url has always been an
// ordinary https:// URL in the System Config contract, so applying the same
// key-only rule to it silently dropped every configured logo from generated
// cards. It gets its own loader instead: still fetched, but only to a public
// host, with the destination address re-checked at connect time so a DNS answer
// cannot redirect the request inward after validation.

const (
	cardLogoFetchTimeout = 10 * time.Second
	cardLogoMaxBytes     = 5 << 20
)

// loadCardLogoImage resolves the configured application logo to image bytes.
// A stored object key (or /files/ proxy URL) is read from our own storage; an
// http(s) URL is fetched under the restrictions above. Any failure returns nil
// — a missing logo must never fail card generation (FR-21).
func (s *Service) loadCardLogoImage(ctx context.Context, stored string) []byte {
	if stored == "" {
		return nil
	}
	if key := avatarKeyFromStored(stored); key != "" {
		return s.loadCardAvatarImage(ctx, stored)
	}
	data, err := fetchPublicImage(ctx, stored)
	if err != nil {
		return nil
	}
	return data
}

// fetchPublicImage GETs an image from a public http(s) URL, refusing any
// request that would reach a private, loopback, link-local, or otherwise
// internal address.
func fetchPublicImage(ctx context.Context, raw string) ([]byte, error) {
	return fetchImageWithDialGuard(ctx, raw, isPublicIP)
}

// fetchImageWithDialGuard is fetchPublicImage with the address policy injected.
// The guard runs at connect time, on every hop of a redirect chain, so a DNS
// answer cannot point the request inward after the URL has been validated.
// Tests supply their own guard because httptest servers bind to loopback, which
// the real policy refuses.
func fetchImageWithDialGuard(ctx context.Context, raw string, allow func(net.IP) bool) ([]byte, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return nil, err
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return nil, fmt.Errorf("unsupported scheme %q", u.Scheme)
	}
	if u.Host == "" {
		return nil, fmt.Errorf("missing host")
	}

	client := &http.Client{
		Timeout: cardLogoFetchTimeout,
		Transport: &http.Transport{
			DialContext: (&net.Dialer{
				Timeout: cardLogoFetchTimeout,
				Control: func(network, address string, _ syscall.RawConn) error {
					host, _, err := net.SplitHostPort(address)
					if err != nil {
						return err
					}
					ip := net.ParseIP(host)
					if ip == nil || !allow(ip) {
						return fmt.Errorf("refusing to connect to non-public address %s", host)
					}
					return nil
				},
			}).DialContext,
		},
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, raw, nil)
	if err != nil {
		return nil, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("logo fetch: status %d", resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); ct != "" && !strings.HasPrefix(ct, "image/") {
		return nil, fmt.Errorf("logo fetch: content-type %q is not an image", ct)
	}

	data, err := io.ReadAll(io.LimitReader(resp.Body, cardLogoMaxBytes))
	if err != nil {
		return nil, err
	}
	if _, ok := decodeImageMime(data); !ok {
		return nil, fmt.Errorf("logo fetch: body is not a decodable image")
	}
	return data, nil
}

const fallbackImageMime = "image/png"

// decodeImageMime sniffs the real mime of image bytes by decoding just the
// header, so callers don't have to trust an upload's declared content-type.
func decodeImageMime(data []byte) (mime string, ok bool) {
	if len(data) == 0 {
		return "", false
	}
	cfg, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil || cfg.Width == 0 || cfg.Height == 0 {
		return "", false
	}
	switch format {
	case "png":
		return "image/png", true
	case "jpeg":
		return "image/jpeg", true
	case "gif":
		return "image/gif", true
	default:
		return "", false
	}
}

// imageMimeOrFallback resolves the real mime of embedded image bytes. Uploads
// are accepted as any image type (the picker is not restricted to PNG), so
// hardcoding image/png here would mislabel every JPEG background.
func imageMimeOrFallback(data []byte) string {
	if mime, ok := decodeImageMime(data); ok {
		return mime
	}
	return fallbackImageMime
}

// isPublicIP reports whether ip is routable on the public internet — the only
// kind of address the logo fetch is allowed to reach.
func isPublicIP(ip net.IP) bool {
	if ip.IsLoopback() || ip.IsUnspecified() || ip.IsPrivate() ||
		ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() ||
		ip.IsInterfaceLocalMulticast() || ip.IsMulticast() {
		return false
	}
	// 169.254.169.254 (cloud metadata) is link-local and already covered; this
	// additionally rejects the IPv6 unique-local range fc00::/7.
	if len(ip) == net.IPv6len && ip.To4() == nil && ip[0]&0xfe == 0xfc {
		return false
	}
	return true
}
