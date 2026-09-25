package api

import (
	"context"
	"testing"
	"time"
)

func TestWaitNotifyIdle(t *testing.T) {
	s := &Server{}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	s.WaitNotify(ctx)
}

func TestWaitNotifyRespectsContext(t *testing.T) {
	s := &Server{}
	s.notifyWG.Add(1)
	defer s.notifyWG.Done()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()
	start := time.Now()
	s.WaitNotify(ctx)
	if time.Since(start) > 300*time.Millisecond {
		t.Fatal("WaitNotify did not return when ctx expired")
	}
}
