package web

import "errors"

var (
	errBadComponent   = errors.New("unknown orchestrator component")
	errRequiredFields = errors.New("required fields missing")
)

func moduleDisabled(name string) error {
	return errors.New(name + " module is disabled")
}
