package runcontrol

import (
	"context"
	"sync"
)

type Controller struct {
	mu      sync.Mutex
	cancels map[string]context.CancelFunc
}

func New() *Controller {
	return &Controller{cancels: make(map[string]context.CancelFunc)}
}

func (c *Controller) Register(runID string, cancel context.CancelFunc) func() {
	c.mu.Lock()
	c.cancels[runID] = cancel
	c.mu.Unlock()
	return func() {
		c.mu.Lock()
		delete(c.cancels, runID)
		c.mu.Unlock()
	}
}

func (c *Controller) Cancel(runID string) bool {
	c.mu.Lock()
	cancel := c.cancels[runID]
	c.mu.Unlock()
	if cancel == nil {
		return false
	}
	cancel()
	return true
}
