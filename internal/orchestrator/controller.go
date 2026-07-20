package orchestrator

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/ai4next/superman/internal/bus"
)

const defaultReconcileInterval = time.Second

type PlanStore interface {
	Save(Plan) error
	Load(string) (Plan, error)
	List() ([]Plan, error)
}

// Controller persists plan state around each reconcile so work can resume after
// a process restart and downstream DAG nodes are released as work completes.
type Controller struct {
	Store   PlanStore
	Queue   bus.TaskQueue
	OnError func(error)

	mu sync.Mutex
}

func NewController(store PlanStore, queue bus.TaskQueue) *Controller {
	return &Controller{Store: store, Queue: queue}
}

func (c *Controller) Submit(plan Plan) (ReconcileResult, error) {
	if err := c.validate(); err != nil {
		return ReconcileResult{}, err
	}
	if err := ValidatePlan(plan); err != nil {
		return ReconcileResult{}, err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if err := c.Store.Save(plan); err != nil {
		return ReconcileResult{}, err
	}
	return c.reconcilePlanLocked(plan.ID)
}

func (c *Controller) ReconcilePlan(id string) (ReconcileResult, error) {
	if err := c.validate(); err != nil {
		return ReconcileResult{}, err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.reconcilePlanLocked(id)
}

func (c *Controller) ReconcileActive() error {
	if err := c.validate(); err != nil {
		return err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	plans, err := c.Store.List()
	if err != nil {
		return err
	}
	var errs []error
	for _, plan := range plans {
		if isTerminalPlan(plan.Status) {
			continue
		}
		if _, err := c.reconcilePlanLocked(plan.ID); err != nil {
			errs = append(errs, fmt.Errorf("reconcile plan %s: %w", plan.ID, err))
		}
	}
	return joinErrors(errs)
}

func (c *Controller) Run(ctx context.Context, interval time.Duration) {
	if interval <= 0 {
		interval = defaultReconcileInterval
	}
	c.report(c.ReconcileActive())
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			c.report(c.ReconcileActive())
		}
	}
}

func (c *Controller) reconcilePlanLocked(id string) (ReconcileResult, error) {
	plan, err := c.Store.Load(id)
	if err != nil {
		return ReconcileResult{}, err
	}
	result, reconcileErr := Reconcile(&plan, c.Queue)
	saveErr := c.Store.Save(plan)
	if reconcileErr != nil && saveErr != nil {
		return result, fmt.Errorf("reconcile plan: %v; save plan: %w", reconcileErr, saveErr)
	}
	if reconcileErr != nil {
		return result, reconcileErr
	}
	return result, saveErr
}

func (c *Controller) validate() error {
	if c == nil || c.Store == nil {
		return fmt.Errorf("plan store is required")
	}
	if c.Queue == nil {
		return fmt.Errorf("task queue is required")
	}
	return nil
}

func (c *Controller) report(err error) {
	if err != nil && c.OnError != nil {
		c.OnError(err)
	}
}

func isTerminalPlan(status PlanStatus) bool {
	return status == PlanStatusDone || status == PlanStatusFailed
}

func joinErrors(errs []error) error {
	if len(errs) == 0 {
		return nil
	}
	message := errs[0]
	for _, err := range errs[1:] {
		message = fmt.Errorf("%v; %w", message, err)
	}
	return message
}
