package shortener

import "testing"

func TestNormalizeCode(t *testing.T) {
	code, err := NormalizeCode("abc12")
	if err != nil || code != "abc12" {
		t.Fatalf("NormalizeCode explicit = %q, %v", code, err)
	}
	auto, err := NormalizeCode("")
	if err != nil || len(auto) != 6 {
		t.Fatalf("NormalizeCode auto = %q, %v", auto, err)
	}
	if _, err := NormalizeCode("bad code!"); err == nil {
		t.Fatal("expected error for invalid code")
	}
}

func TestValidateTarget(t *testing.T) {
	u, err := ValidateTarget("https://example.com/path")
	if err != nil || u != "https://example.com/path" {
		t.Fatalf("ValidateTarget = %q, %v", u, err)
	}
	for _, bad := range []string{"", "ftp://x.com", "not-a-url"} {
		if _, err := ValidateTarget(bad); err == nil {
			t.Fatalf("ValidateTarget(%q) want error", bad)
		}
	}
}
