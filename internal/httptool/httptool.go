package httptool

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Request is a saved HTTP request (Postman-style).
type Request struct {
	Method  string
	URL     string
	Headers map[string]string
	Body    string
}

// Response is the result of executing a request.
type Response struct {
	StatusCode int
	Status     string
	Headers    http.Header
	Body       string
	Duration   time.Duration
	Error      string
}

// Execute performs an HTTP request server-side.
func Execute(ctx context.Context, req Request) Response {
	start := time.Now()
	method := strings.ToUpper(strings.TrimSpace(req.Method))
	if method == "" {
		method = http.MethodGet
	}
	url := strings.TrimSpace(req.URL)
	if url == "" {
		return Response{Error: "URL required", Duration: time.Since(start)}
	}
	var body io.Reader
	if req.Body != "" && method != http.MethodGet && method != http.MethodHead {
		body = strings.NewReader(req.Body)
	}
	httpReq, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return Response{Error: err.Error(), Duration: time.Since(start)}
	}
	for k, v := range req.Headers {
		if strings.TrimSpace(k) != "" {
			httpReq.Header.Set(k, v)
		}
	}
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(httpReq)
	if err != nil {
		return Response{Error: err.Error(), Duration: time.Since(start)}
	}
	defer resp.Body.Close()
	const maxBody = 256 * 1024
	b, _ := io.ReadAll(io.LimitReader(resp.Body, maxBody))
	out := Response{
		StatusCode: resp.StatusCode,
		Status:     resp.Status,
		Headers:    resp.Header.Clone(),
		Body:       string(b),
		Duration:   time.Since(start),
	}
	if len(b) >= maxBody {
		out.Body += fmt.Sprintf("\n\n… truncated at %d bytes", maxBody)
	}
	return out
}

// ParseHeaders parses "Key: Value" lines.
func ParseHeaders(raw string) map[string]string {
	out := map[string]string{}
	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		i := strings.Index(line, ":")
		if i <= 0 {
			continue
		}
		out[strings.TrimSpace(line[:i])] = strings.TrimSpace(line[i+1:])
	}
	return out
}
