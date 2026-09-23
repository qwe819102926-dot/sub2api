package service

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/pkg/httpclient"
	"github.com/stretchr/testify/require"
)

func TestGeneratedImageDownloadRejectsUnsafeURLs(t *testing.T) {
	svc := NewGeneratedImageDownloadService()
	for _, rawURL := range []string{
		"",
		"not a url",
		"file:///etc/passwd",
		"ftp://example.com/a.png",
		"http://user:pass@cdn.example.com/a.png",
		"http://127.0.0.1/a.png",
		"http://localhost/a.png",
		"http://10.1.2.3/a.png",
		"http://192.168.1.8/a.png",
		"http://[::1]/a.png",
		"http://169.254.169.254/latest/meta-data",
		"http://metadata.google.internal/computeMetadata/v1/",
		"https://metadata.goog/",
		"http://instance-data.ec2.internal/",
	} {
		t.Run(rawURL, func(t *testing.T) {
			_, err := svc.Download(context.Background(), rawURL)
			require.ErrorIs(t, err, ErrGeneratedImageURLRejected)
		})
	}
}

func TestGeneratedImageDownloadAcceptsImageSignatures(t *testing.T) {
	png := []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n', 0, 0}
	jpeg := []byte{0xff, 0xd8, 0xff, 0xe0, 0, 0}
	gif := []byte("GIF89a")
	webp := append([]byte("RIFF\x00\x00\x00\x00WEBP"), 0, 0)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/png":
			w.Header().Set("Content-Type", "text/plain")
			_, _ = w.Write(png)
		case "/jpeg":
			_, _ = w.Write(jpeg)
		case "/gif":
			_, _ = w.Write(gif)
		case "/webp":
			_, _ = w.Write(webp)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(upstream.Close)

	svc := NewGeneratedImageDownloadServiceForTest(&http.Client{})
	for _, tc := range []struct {
		path        string
		contentType string
		filename    string
	}{
		{path: "/png", contentType: "image/png", filename: "generated.png"},
		{path: "/jpeg", contentType: "image/jpeg", filename: "generated.jpg"},
		{path: "/gif", contentType: "image/gif", filename: "generated.gif"},
		{path: "/webp", contentType: "image/webp", filename: "generated.webp"},
	} {
		t.Run(tc.path, func(t *testing.T) {
			got, err := svc.Download(context.Background(), upstream.URL+tc.path)
			require.NoError(t, err)
			require.Equal(t, tc.contentType, got.ContentType)
			require.Equal(t, tc.filename, got.Filename)
		})
	}
}

func TestGeneratedImageDownloadRejectsNonImagesAndOversizedBodies(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/html":
			w.Header().Set("Content-Type", "image/png")
			_, _ = w.Write([]byte("<!DOCTYPE html><html></html>"))
		case "/svg":
			_, _ = w.Write([]byte("<svg xmlns=\"http://www.w3.org/2000/svg\"></svg>"))
		case "/huge":
			w.Header().Set("Content-Length", strconv.FormatInt(generatedImageMaxDownloadBytes+1, 10))
			_, _ = w.Write([]byte{0x89, 'P', 'N', 'G'})
		case "/status":
			http.Error(w, "nope", http.StatusBadGateway)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(upstream.Close)
	svc := NewGeneratedImageDownloadServiceForTest(&http.Client{})

	_, err := svc.Download(context.Background(), upstream.URL+"/html")
	require.ErrorIs(t, err, ErrGeneratedImageNotAllowed)
	_, err = svc.Download(context.Background(), upstream.URL+"/svg")
	require.ErrorIs(t, err, ErrGeneratedImageNotAllowed)
	_, err = svc.Download(context.Background(), upstream.URL+"/huge")
	require.ErrorIs(t, err, ErrGeneratedImageTooLarge)
	_, err = svc.Download(context.Background(), upstream.URL+"/status")
	require.ErrorIs(t, err, ErrGeneratedImageFetchFailed)

	png := []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n'}
	limited := NewGeneratedImageDownloadServiceForTest(&http.Client{})
	limited.maxBytes = 4
	small := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(png)
	}))
	t.Cleanup(small.Close)
	_, err = limited.Download(context.Background(), small.URL)
	require.ErrorIs(t, err, ErrGeneratedImageTooLarge)
}

func TestGeneratedImageDownloadRejectsMetadataRedirect(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "http://169.254.169.254/latest/meta-data", http.StatusFound)
	}))
	t.Cleanup(upstream.Close)

	svc := NewGeneratedImageDownloadServiceForTest(&http.Client{})
	_, err := svc.Download(context.Background(), upstream.URL+"/image")
	require.ErrorIs(t, err, ErrGeneratedImageURLRejected)
}

func TestGeneratedImageDownloadDoesNotMutateSharedClient(t *testing.T) {
	svc := NewGeneratedImageDownloadService()
	shared, err := httpclient.GetClient(generatedImageDownloadClientOptions())
	require.NoError(t, err)
	require.Nil(t, shared.CheckRedirect)
	require.NotNil(t, svc.client.CheckRedirect)
	require.NotSame(t, shared, svc.client)
}

func TestAllowedImageMagicDoesNotDefaultToPNG(t *testing.T) {
	contentType, ok := allowedImageMagic([]byte("<svg></svg>"))
	require.False(t, ok)
	require.Empty(t, contentType)
	contentType, ok = allowedImageMagic(nil)
	require.False(t, ok)
	require.Empty(t, contentType)
}
