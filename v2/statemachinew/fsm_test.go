package statemachinew

import (
	"context"
	"errors"
	"testing"
)

// Mock entity to test transitions
type TestOrder struct {
	ID        string
	Balance   int
	LogOutput []string
}

const (
	StatePending State = "PENDING"
	StatePaid    State = "PAID"
	StateShipped State = "SHIPPED"

	EventPay  Event = "PAY"
	EventShip Event = "SHIP"
)

func TestFSM_SuccessfulTransition(t *testing.T) {
	sm := New()
	ctx := context.Background()

	sm.On(EventPay).
		From(StatePending).
		To(StatePaid).
		Guard(func(ctx context.Context, entity any) error {
			order := entity.(*TestOrder)
			if order.Balance < 100 {
				return errors.New("insufficient balance")
			}
			order.LogOutput = append(order.LogOutput, "GuardPassed")
			return nil
		}).
		Pre(func(ctx context.Context, entity any) error {
			order := entity.(*TestOrder)
			order.LogOutput = append(order.LogOutput, "PreHookExecuted")
			return nil
		}).
		Post(func(ctx context.Context, entity any) error {
			order := entity.(*TestOrder)
			order.LogOutput = append(order.LogOutput, "PostHookExecuted")
			return nil
		})

	order := &TestOrder{ID: "123", Balance: 150}

	newState, err := sm.Fire(ctx, EventPay, StatePending, order)
	if err != nil {
		t.Fatalf("Expected transition to succeed, got error: %v", err)
	}

	if newState != StatePaid {
		t.Errorf("Expected new state to be %s, got %s", StatePaid, newState)
	}

	// Verify Execution Order
	expectedLogs := []string{"GuardPassed", "PreHookExecuted", "PostHookExecuted"}
	for i, log := range expectedLogs {
		if order.LogOutput[i] != log {
			t.Errorf("Expected execution log %s at index %d, got %s", log, i, order.LogOutput[i])
		}
	}
}

func TestFSM_InvalidTransition(t *testing.T) {
	sm := New()
	ctx := context.Background()

	// Rule: Can only ship if currently PAID
	sm.On(EventShip).From(StatePaid).To(StateShipped)

	order := &TestOrder{ID: "123"}

	// Attempt to ship from PENDING (Should fail)
	newState, err := sm.Fire(ctx, EventShip, StatePending, order)

	if err == nil {
		t.Fatal("Expected an error for invalid transition, got nil")
	}

	if !errors.Is(err, ErrInvalidTransition) {
		t.Errorf("Expected ErrInvalidTransition, got %v", err)
	}

	if newState != StatePending {
		t.Errorf("State should not change on failure. Expected %s, got %s", StatePending, newState)
	}
}

func TestFSM_GuardFailure(t *testing.T) {
	sm := New()
	ctx := context.Background()

	sm.On(EventPay).
		From(StatePending).
		To(StatePaid).
		Guard(func(ctx context.Context, entity any) error {
			return errors.New("payment gateway offline")
		}).
		Pre(func(ctx context.Context, entity any) error {
			t.Fatal("Pre hook should never execute if Guard fails")
			return nil
		})

	order := &TestOrder{ID: "123"}

	newState, err := sm.Fire(ctx, EventPay, StatePending, order)

	if err == nil {
		t.Fatal("Expected transition to fail due to guard, got nil")
	}

	if !errors.Is(err, ErrTransitionAborted) {
		t.Errorf("Expected ErrTransitionAborted, got %v", err)
	}

	if newState != StatePending {
		t.Errorf("State should remain %s, got %s", StatePending, newState)
	}
}

func TestFSM_UnknownEvent(t *testing.T) {
	sm := New()
	ctx := context.Background()

	order := &TestOrder{}

	newState, err := sm.Fire(ctx, Event("UNKNOWN_EVENT"), StatePending, order)

	if err == nil {
		t.Fatal("Expected an error for an unknown event, got nil")
	}

	if newState != StatePending {
		t.Errorf("State should remain unchanged on unknown event")
	}
}
