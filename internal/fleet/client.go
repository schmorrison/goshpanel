package fleet

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Client calls remote GoshPanel fleet APIs.
type Client struct {
	http *http.Client
}

// NewClient creates an HTTP fleet client.
func NewClient() *Client {
	return &Client{http: &http.Client{Timeout: 15 * time.Second}}
}

// FetchTelemetry GETs /api/v1/fleet/telemetry from a remote node.
func (c *Client) FetchTelemetry(ctx context.Context, baseURL, token string) (Telemetry, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, joinURL(baseURL, "/api/v1/fleet/telemetry"), nil)
	if err != nil {
		return Telemetry{}, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := c.http.Do(req)
	if err != nil {
		return Telemetry{}, fmt.Errorf("fetch telemetry: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return Telemetry{}, fmt.Errorf("telemetry HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	var t Telemetry
	if err := json.NewDecoder(resp.Body).Decode(&t); err != nil {
		return Telemetry{}, fmt.Errorf("decode telemetry: %w", err)
	}
	return t, nil
}

// SendControl POSTs a control action to a remote node.
func (c *Client) SendControl(ctx context.Context, baseURL, token string, ctrl ControlRequest) (ControlResponse, error) {
	body, _ := json.Marshal(ctrl)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, joinURL(baseURL, "/api/v1/fleet/control"), bytes.NewReader(body))
	if err != nil {
		return ControlResponse{}, err
	}
	httpReq.Header.Set("Authorization", "Bearer "+token)
	httpReq.Header.Set("Content-Type", "application/json")
	resp, err := c.http.Do(httpReq)
	if err != nil {
		return ControlResponse{}, fmt.Errorf("send control: %w", err)
	}
	defer resp.Body.Close()
	var out ControlResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		b, _ := io.ReadAll(resp.Body)
		return ControlResponse{}, fmt.Errorf("decode control response: %s", b)
	}
	if resp.StatusCode != http.StatusOK {
		return out, fmt.Errorf("control HTTP %d: %s", resp.StatusCode, out.Message)
	}
	return out, nil
}

// Enroll POSTs to /api/v1/fleet/enroll and returns per-node credentials.
func (c *Client) Enroll(ctx context.Context, controllerURL, enrollToken, nodeName, baseURL string) (EnrollResponse, error) {
	body, _ := json.Marshal(EnrollRequest{
		EnrollToken: enrollToken,
		NodeName:    nodeName,
		BaseURL:     baseURL,
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, joinURL(controllerURL, "/api/v1/fleet/enroll"), bytes.NewReader(body))
	if err != nil {
		return EnrollResponse{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return EnrollResponse{}, fmt.Errorf("enroll: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return EnrollResponse{}, fmt.Errorf("enroll HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(b)))
	}
	var out EnrollResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return EnrollResponse{}, fmt.Errorf("decode enroll response: %w", err)
	}
	return out, nil
}

// PushTelemetry POSTs telemetry to a controller ingest endpoint.
func (c *Client) PushTelemetry(ctx context.Context, controllerURL, token string, ingest IngestRequest) error {
	body, _ := json.Marshal(ingest)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, joinURL(controllerURL, "/api/v1/fleet/ingest"), bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("push telemetry: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("ingest HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(b)))
	}
	return nil
}

func joinURL(base, path string) string {
	return strings.TrimRight(base, "/") + path
}
