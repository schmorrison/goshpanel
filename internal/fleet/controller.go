package fleet

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/schmorrison/goshpanel/internal/store"
)

// Controller manages registered fleet nodes on the main instance.
type Controller struct {
	store  *store.Store
	client *Client
}

// NewController creates a fleet controller.
func NewController(st *store.Store) *Controller {
	return &Controller{store: st, client: NewClient()}
}

// PollNode fetches telemetry from one node and stores it.
func (c *Controller) PollNode(ctx context.Context, node store.FleetNode) error {
	t, err := c.client.FetchTelemetry(ctx, node.BaseURL, node.Token)
	if err != nil {
		_ = c.store.SetFleetNodeError(node.ID, err.Error())
		return err
	}
	return c.ingest(node.ID, t)
}

// PollAll polls every enabled registered node.
func (c *Controller) PollAll(ctx context.Context) []error {
	nodes, err := c.store.FleetNodes()
	if err != nil {
		return []error{err}
	}
	var errs []error
	for _, n := range nodes {
		if !n.Enabled {
			continue
		}
		if err := c.PollNode(ctx, n); err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", n.Name, err))
		}
	}
	return errs
}

// IngestPush handles a worker pushing telemetry to the controller.
func (c *Controller) IngestPush(nodeName string, t Telemetry) error {
	node, err := c.store.FleetNodeByName(nodeName)
	if err != nil {
		return fmt.Errorf("unknown node %q: register it on the controller first", nodeName)
	}
	return c.ingest(node.ID, t)
}

func (c *Controller) ingest(nodeID int64, t Telemetry) error {
	data, err := json.Marshal(t)
	if err != nil {
		return err
	}
	if err := c.store.RecordFleetTelemetry(nodeID, data); err != nil {
		return err
	}
	return c.store.SetFleetNodeSeen(nodeID, time.Now().UTC())
}

// SendCommand dispatches a control action to a remote node.
func (c *Controller) SendCommand(ctx context.Context, nodeID int64, req ControlRequest) (ControlResponse, error) {
	node, err := c.store.FleetNodeByID(nodeID)
	if err != nil {
		return ControlResponse{}, err
	}
	cmd, err := c.store.CreateFleetCommand(nodeID, req.Action)
	if err != nil {
		return ControlResponse{}, err
	}
	res, err := c.client.SendControl(ctx, node.BaseURL, node.Token, req)
	status := "completed"
	result := res.Message
	if err != nil {
		status = "failed"
		result = err.Error()
	} else if !res.OK {
		status = "failed"
	}
	_ = c.store.CompleteFleetCommand(cmd.ID, status, result)
	if err != nil {
		return res, err
	}
	return res, nil
}
