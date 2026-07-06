package fleet

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/schmorrison/goshpanel/internal/config"
	"github.com/schmorrison/goshpanel/internal/crypto"
	"github.com/schmorrison/goshpanel/internal/store"
)

// ErrNotEnrolled is returned when a worker has no persisted fleet credentials.
var ErrNotEnrolled = errors.New("fleet agent not enrolled")

// EnrollRequest is sent by a worker to join a fleet.
type EnrollRequest struct {
	EnrollToken string `json:"enroll_token"`
	NodeName    string `json:"node_name"`
	BaseURL     string `json:"base_url"`
}

// EnrollResponse is returned after successful enrollment.
type EnrollResponse struct {
	NodeToken     string `json:"node_token"`
	NodeName      string `json:"node_name"`
	ControllerURL string `json:"controller_url"`
}

// EnrollSecret returns the HMAC secret used to sign enrollment JWTs.
func EnrollSecret(cfg config.Config) string {
	if s := strings.TrimSpace(cfg.FleetEnrollSecret); s != "" {
		return s
	}
	return strings.TrimSpace(cfg.FleetToken)
}

// EnrollController registers or updates a worker after validating its enroll JWT.
func EnrollController(st *store.Store, secret, controllerURL string, req EnrollRequest) (EnrollResponse, error) {
	claims, err := VerifyEnrollToken(secret, strings.TrimSpace(req.EnrollToken))
	if err != nil {
		return EnrollResponse{}, err
	}
	name := strings.TrimSpace(req.NodeName)
	if name == "" && claims.Name != "" {
		name = claims.Name
	}
	if name == "" {
		return EnrollResponse{}, errors.New("node_name required")
	}
	baseURL := strings.TrimRight(strings.TrimSpace(req.BaseURL), "/")
	if baseURL == "" {
		return EnrollResponse{}, errors.New("base_url required")
	}
	nodeToken, err := crypto.RandomToken()
	if err != nil {
		return EnrollResponse{}, err
	}
	if _, err := st.UpsertFleetNode(name, baseURL, nodeToken); err != nil {
		return EnrollResponse{}, err
	}
	return EnrollResponse{
		NodeToken:     nodeToken,
		NodeName:      name,
		ControllerURL: strings.TrimRight(strings.TrimSpace(controllerURL), "/"),
	}, nil
}

// EnrollWorker calls the controller enroll API and persists returned credentials.
func EnrollWorker(ctx context.Context, cfg config.Config, client *Client) (AgentCredentials, error) {
	controllerURL := strings.TrimRight(strings.TrimSpace(cfg.FleetControllerURL), "/")
	if controllerURL == "" {
		if creds, err := LoadAgentCredentials(cfg.DataDir); err == nil {
			controllerURL = creds.ControllerURL
		}
	}
	if controllerURL == "" {
		return AgentCredentials{}, errors.New("controller URL required for enrollment")
	}
	rt := ResolveWorkerRuntime(cfg)
	res, err := client.Enroll(ctx, controllerURL, cfg.FleetEnrollToken, rt.NodeName, rt.BaseURL)
	if err != nil {
		return AgentCredentials{}, err
	}
	creds := AgentCredentials{
		NodeName:      res.NodeName,
		NodeToken:     res.NodeToken,
		ControllerURL: res.ControllerURL,
		EnrolledAt:    time.Now().UTC(),
	}
	if creds.ControllerURL == "" {
		creds.ControllerURL = controllerURL
	}
	if err := SaveAgentCredentials(cfg.DataDir, creds); err != nil {
		return AgentCredentials{}, fmt.Errorf("save agent credentials: %w", err)
	}
	return creds, nil
}

// DefaultEnrollTTL is how long UI-generated enrollment tokens remain valid.
const DefaultEnrollTTL = 7 * 24 * time.Hour
