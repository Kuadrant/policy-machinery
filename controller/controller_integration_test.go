//go:build integration

package controller

import (
	"context"
	"testing"
	"time"
)

func TestStartControllerManaged(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	testManager, err := buildStartableTestManager()
	if err != nil {
		t.Errorf("expected no error when building test manager, got %s", err.Error())
	}
	c := NewController(ManagedBy(testManager))
	go func() {
		if err := c.Start(ctx); err != nil {
			t.Errorf("expected no error when starting manager, got %s", err.Error())
			cancel()
		}
	}()
	time.Sleep(3 * time.Second)
}
