package service

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/httpclient"
)

const generatedImageMaxDownloadBytes int64 = 32 << 20

var (
	ErrGeneratedImageURLRejected = errors.New("generated image url rejected")
	ErrGeneratedImageTooLarge    = errors.New("generated image too large")
	ErrGeneratedImageNotAllowed  = errors.New("generated image type not allowed")
	ErrGeneratedImageFetchFailed = errors.New("generated image fetch failed")
)

// GeneratedImageDownload is a sniffed image fetched for the browser download proxy.
type GeneratedImageDownload struct {
	Data        []byte
	ContentType string
	Filename    string
}

// GeneratedImageDownloadService fetches a public generated-image URL.
// It is not a general URL proxy: only http(s) targets are accepted, and the
// body must match a PNG, JPEG, GIF, or WebP signature.
type GeneratedImageDownloadService struct {
	client       *http.Client
	maxBytes     int64
	allowPrivate bool
}

// NewGeneratedImageDownloadService builds a production downloader.
// The shared HTTP client is copied so redirect checks stay local to this service.
func NewGeneratedImageDownloadService() *GeneratedImageDownloadService {
	client, err := httpclient.GetClient(generatedImageDownloadClientOptions())
	if err != nil || client == nil {
		client = &http.Client{Transport: roundTripperFunc(func(*http.Request) (*http.Response, error) {
			if err != nil {
				return nil, err
			}
			return nil, errors.New("generated image download client is unavailable")
		})}
	}
	copied := *client
	svc := &GeneratedImageDownloadService{
		maxBytes: generatedImageMaxDownloadBytes,
	}
	copied.CheckRedirect = svc.checkRedirect
	svc.client = &copied
	return svc
}

// NewGeneratedImageDownloadServiceForTest skips loopback and RFC1918 host checks
// so httptest servers on 127.0.0.1 can be used. Link-local and metadata targets stay blocked.
func NewGeneratedImageDownloadServiceForTest(client *http.Client) *GeneratedImageDownloadService {
	if client == nil {
		client = &http.Client{Timeout: 5 * time.Second}
	}
	copied := *client
	svc := &GeneratedImageDownloadService{
		maxBytes:     generatedImageMaxDownloadBytes,
		allowPrivate: true,
	}
	copied.CheckRedirect = svc.checkRedirect
	svc.client = &copied
	return svc
}

func generatedImageDownloadClientOptions() httpclient.Options {
	return httpclient.Options{
		Timeout:               60 * time.Second,
		ResponseHeaderTimeout: 20 * time.Second,
		PinResolvedIP:         true,
	}
}

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func (s *GeneratedImageDownloadService) Download(ctx context.Context, rawURL string) (*GeneratedImageDownload, error) {
	if s == nil || s.client == nil {
		return nil, ErrGeneratedImageFetchFailed
	}
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return nil, ErrGeneratedImageURLRejected
	}
	if err := s.validateURL(parsed); err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, parsed.String(), nil)
	if err != nil {
		return nil, ErrGeneratedImageURLRejected
	}
	req.Header.Set("Accept", "image/png,image/jpeg,image/webp,image/gif")
	resp, err := s.client.Do(req)
	if err != nil {
		if errors.Is(err, ErrGeneratedImageURLRejected) || errors.Is(err, ErrGeneratedImageFetchFailed) {
			return nil, err
		}
		return nil, fmt.Errorf("%w", ErrGeneratedImageFetchFailed)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("%w: status %d", ErrGeneratedImageFetchFailed, resp.StatusCode)
	}
	limit := s.maxBytes
	if limit <= 0 {
		limit = generatedImageMaxDownloadBytes
	}
	if resp.ContentLength > limit {
		return nil, ErrGeneratedImageTooLarge
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, limit+1))
	if err != nil {
		return nil, fmt.Errorf("%w", ErrGeneratedImageFetchFailed)
	}
	if int64(len(data)) > limit {
		return nil, ErrGeneratedImageTooLarge
	}
	contentType, ok := allowedImageMagic(data)
	if !ok {
		return nil, ErrGeneratedImageNotAllowed
	}
	return &GeneratedImageDownload{
		Data:        data,
		ContentType: contentType,
		Filename:    filenameForGeneratedImage(contentType),
	}, nil
}

func (s *GeneratedImageDownloadService) checkRedirect(req *http.Request, via []*http.Request) error {
	if len(via) >= 3 {
		return ErrGeneratedImageFetchFailed
	}
	if req == nil || req.URL == nil {
		return ErrGeneratedImageURLRejected
	}
	return s.validateURL(req.URL)
}

func (s *GeneratedImageDownloadService) validateURL(parsed *url.URL) error {
	if parsed == nil || parsed.Host == "" {
		return ErrGeneratedImageURLRejected
	}
	scheme := strings.ToLower(parsed.Scheme)
	if scheme != "http" && scheme != "https" {
		return ErrGeneratedImageURLRejected
	}
	if parsed.User != nil {
		return ErrGeneratedImageURLRejected
	}
	host := strings.TrimSuffix(strings.ToLower(parsed.Hostname()), ".")
	if host == "" || generatedImageHostBlocked(host, s != nil && s.allowPrivate) {
		return ErrGeneratedImageURLRejected
	}
	return nil
}

func generatedImageHostBlocked(host string, allowPrivate bool) bool {
	if isGeneratedImageMetadataHost(host) {
		return true
	}
	if host == "localhost" || host == "localhost.localdomain" || strings.HasSuffix(host, ".localhost") {
		return !allowPrivate
	}
	ip := net.ParseIP(host)
	if ip == nil {
		return false
	}
	if ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsUnspecified() || ip.IsInterfaceLocalMulticast() {
		return true
	}
	if !allowPrivate && (ip.IsLoopback() || ip.IsPrivate()) {
		return true
	}
	return false
}

func isGeneratedImageMetadataHost(host string) bool {
	switch host {
	case "metadata", "metadata.google.internal", "metadata.goog", "instance-data", "instance-data.ec2.internal":
		return true
	}
	return strings.HasPrefix(host, "metadata.")
}

// allowedImageMagic accepts only image signatures. It intentionally does not
// fall back to image/png the way detectImageContentType does.
func allowedImageMagic(data []byte) (string, bool) {
	if len(data) >= 8 && data[0] == 0x89 && data[1] == 'P' && data[2] == 'N' && data[3] == 'G' && data[4] == '\r' && data[5] == '\n' && data[6] == 0x1a && data[7] == '\n' {
		return "image/png", true
	}
	if len(data) >= 3 && data[0] == 0xff && data[1] == 0xd8 && data[2] == 0xff {
		return "image/jpeg", true
	}
	if len(data) >= 6 && (string(data[:6]) == "GIF87a" || string(data[:6]) == "GIF89a") {
		return "image/gif", true
	}
	if len(data) >= 12 && string(data[:4]) == "RIFF" && string(data[8:12]) == "WEBP" {
		return "image/webp", true
	}
	return "", false
}

func filenameForGeneratedImage(contentType string) string {
	switch contentType {
	case "image/jpeg":
		return "generated.jpg"
	case "image/webp":
		return "generated.webp"
	case "image/gif":
		return "generated.gif"
	default:
		return "generated.png"
	}
}
