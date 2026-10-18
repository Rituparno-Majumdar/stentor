package platforms

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"time"
)

var httpClient = &http.Client{Timeout: 30 * time.Second}

// doRequest is the one place every platform file routes its outbound call
// through -- single request, no retries, no connection kept open after.
func doRequest(ctx context.Context, method, rawURL string, headers map[string]string, body io.Reader) ([]byte, int, error) {
	req, err := http.NewRequestWithContext(ctx, method, rawURL, body)
	if err != nil {
		return nil, 0, err
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.StatusCode, err
	}
	return data, resp.StatusCode, nil
}

// doJSON marshals `payload`, sends it, and unmarshals a JSON response into
// `out` (pass nil to ignore the body). Returns an error on any non-2xx.
func doJSON(ctx context.Context, method, rawURL string, headers map[string]string, payload, out any) error {
	h := map[string]string{"Content-Type": "application/json"}
	for k, v := range headers {
		h[k] = v
	}
	var body io.Reader
	if payload != nil {
		b, err := json.Marshal(payload)
		if err != nil {
			return err
		}
		body = bytes.NewReader(b)
	}
	data, status, err := doRequest(ctx, method, rawURL, h, body)
	if err != nil {
		return err
	}
	if status < 200 || status >= 300 {
		return fmt.Errorf("http %d: %s", status, truncate(string(data), 500))
	}
	if out != nil && len(data) > 0 {
		if err := json.Unmarshal(data, out); err != nil {
			return fmt.Errorf("decoding response: %w (body: %s)", err, truncate(string(data), 300))
		}
	}
	return nil
}

// doForm posts application/x-www-form-urlencoded data (several OAuth token
// endpoints and the classic Reddit submit API require this, not JSON).
func doForm(ctx context.Context, method, rawURL string, headers map[string]string, form url.Values, out any) error {
	h := map[string]string{"Content-Type": "application/x-www-form-urlencoded"}
	for k, v := range headers {
		h[k] = v
	}
	data, status, err := doRequest(ctx, method, rawURL, h, bytes.NewReader([]byte(form.Encode())))
	if err != nil {
		return err
	}
	if status < 200 || status >= 300 {
		return fmt.Errorf("http %d: %s", status, truncate(string(data), 500))
	}
	if out != nil && len(data) > 0 {
		if err := json.Unmarshal(data, out); err != nil {
			return fmt.Errorf("decoding response: %w (body: %s)", err, truncate(string(data), 300))
		}
	}
	return nil
}

// doMultipart uploads a local file plus a set of plain form fields --
// used by Discord (attachment), Mastodon/Bluesky-style media endpoints.
func doMultipart(ctx context.Context, rawURL string, headers map[string]string, fields map[string]string, fileField, filePath string) ([]byte, int, error) {
	buf := &bytes.Buffer{}
	w := multipart.NewWriter(buf)
	for k, v := range fields {
		if err := w.WriteField(k, v); err != nil {
			return nil, 0, err
		}
	}
	if filePath != "" {
		f, err := os.Open(filePath)
		if err != nil {
			return nil, 0, fmt.Errorf("opening image %s: %w", filePath, err)
		}
		defer f.Close()
		part, err := w.CreateFormFile(fileField, filepath.Base(filePath))
		if err != nil {
			return nil, 0, err
		}
		if _, err := io.Copy(part, f); err != nil {
			return nil, 0, err
		}
	}
	if err := w.Close(); err != nil {
		return nil, 0, err
	}
	h := map[string]string{"Content-Type": w.FormDataContentType()}
	for k, v := range headers {
		h[k] = v
	}
	return doRequest(ctx, http.MethodPost, rawURL, h, buf)
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

func readFile(path string) ([]byte, error) {
	return os.ReadFile(path)
}

func jsonUnmarshal(data []byte, out any) error {
	return json.Unmarshal(data, out)
}

func byteReader(data []byte) io.Reader {
	return bytes.NewReader(data)
}
