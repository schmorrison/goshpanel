package dbmanager

import (
	"testing"
)

func TestValidateIdent(t *testing.T) {
	if err := validateIdent("app_db"); err != nil {
		t.Error(err)
	}
	if err := validateIdent("bad-name"); err == nil {
		t.Error("expected reject")
	}
}
