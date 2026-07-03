package cron

import (
	"strings"
	"testing"

	"github.com/schmorrison/goshpanel/internal/store"
)

func TestValidateSchedule(t *testing.T) {
	valid := []string{
		"* * * * *", "*/5 * * * *", "0 0 * * 0", "15 2 1 6 *",
		"1,15,30 * * * *", "0-30 * * * *", "0-30/2 8-18 * * 1-5",
		"@daily", "@reboot",
	}
	for _, expr := range valid {
		if err := ValidateSchedule(expr); err != nil {
			t.Errorf("ValidateSchedule(%q) = %v, want nil", expr, err)
		}
	}

	invalid := []string{
		"", "* * * *", "60 * * * *", "* 24 * * *", "* * 32 * *",
		"* * * 13 *", "* * * * 8", "a * * * *", "*/0 * * * *",
		"30-10 * * * *", "@nonsense",
	}
	for _, expr := range invalid {
		if err := ValidateSchedule(expr); err == nil {
			t.Errorf("ValidateSchedule(%q) = nil, want error", expr)
		}
	}
}

func TestRenderCrontab(t *testing.T) {
	out := RenderCrontab([]store.CronJob{
		{Schedule: "*/5 * * * *", Command: "echo hi", Comment: "greeting"},
		{Schedule: "@daily", Command: "backup.sh"},
	})
	if !strings.Contains(out, "# greeting\n*/5 * * * * echo hi\n") {
		t.Errorf("missing commented job:\n%s", out)
	}
	if !strings.Contains(out, "@daily backup.sh\n") {
		t.Errorf("missing @daily job:\n%s", out)
	}
}
