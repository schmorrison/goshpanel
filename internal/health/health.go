package health

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/schmorrison/goshpanel/internal/store"
)

// CheckResult is one probe outcome.
type CheckResult struct {
	OK       bool
	Message  string
	Duration time.Duration
}

// Probe performs an HTTP health check.
func Probe(ctx context.Context, check store.HealthCheck) CheckResult {
	start := time.Now()
	target := strings.TrimSpace(check.URL)
	if target == "" {
		return CheckResult{Message: "empty URL", Duration: time.Since(start)}
	}
	method := strings.ToUpper(strings.TrimSpace(check.Method))
	if method == "" {
		method = http.MethodGet
	}
	ctx, cancel := context.WithTimeout(ctx, time.Duration(check.TimeoutSec)*time.Second)
	defer cancel()
	if check.TimeoutSec <= 0 {
		ctx, cancel = context.WithTimeout(ctx, 10*time.Second)
		defer cancel()
	}
	req, err := http.NewRequestWithContext(ctx, method, target, nil)
	if err != nil {
		return CheckResult{Message: err.Error(), Duration: time.Since(start)}
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return CheckResult{Message: err.Error(), Duration: time.Since(start)}
	}
	defer resp.Body.Close()
	want := check.ExpectStatus
	if want <= 0 {
		want = 200
	}
	if resp.StatusCode != want {
		return CheckResult{
			Message:  fmt.Sprintf("status %d want %d", resp.StatusCode, want),
			Duration: time.Since(start),
		}
	}
	return CheckResult{OK: true, Message: resp.Status, Duration: time.Since(start)}
}
