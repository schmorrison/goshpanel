package dkim

import (
	"strings"
	"testing"
)

func TestGenerate(t *testing.T) {
	kp, err := Generate("goshpanel", "example.com")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(kp.PublicDNS, "v=DKIM1") || kp.PrivatePEM == "" {
		t.Errorf("keypair = %+v", kp)
	}
}
