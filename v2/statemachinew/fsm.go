// Package statemachinew provides a stateless, thread-safe Finite State Machine (FSM).
// It acts as a rule engine to manage complex state transitions, ensuring that
// business logic remains clean, predictable, and strictly decoupled from if-else chains.
package statemachinew

import (
	"context"
	"errors"
	"fmt"
	"sync"
)

// State represents the condition of an entity at a given moment.
type State string

// Event represents an action or trigger that initiates a state transition.
type Event string

var (
	// ErrInvalidTransition is returned when an event is fired from an unauthorized state.
	ErrInvalidTransition = errors.New("statemachinew: invalid transition for current state")

	// ErrTransitionAborted is returned when a Guard hook fails, blocking the transition.
	ErrTransitionAborted = errors.New("statemachinew: transition aborted by guard validation")
)

// HookFunc defines the signature for Guards, Pre-actions, and Post-actions.
// The 'entity' parameter is your domain model (e.g., *Order, *User) passed by reference.
type HookFunc func(ctx context.Context, entity any) error

// StateMachine defines the contract for the FSM orchestrator.
type StateMachine interface {
	// On starts building a transition rule for a specific event.
	On(event Event) TransitionBuilder

	// Fire attempts to transition an entity from its currentState using an event.
	// The execution order is: Guards -> Pre-Hooks -> State Change -> Post-Hooks.
	// It returns the new state (or the original state if it failed) and any error encountered.
	Fire(ctx context.Context, event Event, currentState State, entity any) (State, error)
}

// TransitionBuilder provides a fluent API for configuring state transition rules.
type TransitionBuilder interface {
	// From defines the allowed starting states for this transition.
	From(states ...State) TransitionBuilder

	// To defines the final state if the transition succeeds.
	To(state State) TransitionBuilder

	// Guard adds validation checks. If any guard returns an error, the transition is blocked.
	Guard(guards ...HookFunc) TransitionBuilder

	// Pre adds actions executed BEFORE the state officially changes.
	Pre(actions ...HookFunc) TransitionBuilder

	// Post adds actions executed AFTER the state officially changes.
	Post(actions ...HookFunc) TransitionBuilder
}

type fsm struct {
	mu    sync.RWMutex
	rules map[Event]*transitionRule
}

type transitionRule struct {
	fromStates map[State]bool
	toState    State
	guards     []HookFunc
	preHooks   []HookFunc
	postHooks  []HookFunc
}

// New initializes a new, thread-safe State Machine engine.
func New() StateMachine {
	return &fsm{
		rules: make(map[Event]*transitionRule),
	}
}

func (f *fsm) On(event Event) TransitionBuilder {
	f.mu.Lock()
	defer f.mu.Unlock()

	rule := &transitionRule{
		fromStates: make(map[State]bool),
	}
	f.rules[event] = rule
	return &builder{rule: rule}
}

type builder struct {
	rule *transitionRule
}

func (b *builder) From(states ...State) TransitionBuilder {
	for _, s := range states {
		b.rule.fromStates[s] = true
	}
	return b
}

func (b *builder) To(state State) TransitionBuilder {
	b.rule.toState = state
	return b
}

func (b *builder) Guard(guards ...HookFunc) TransitionBuilder {
	b.rule.guards = append(b.rule.guards, guards...)
	return b
}

func (b *builder) Pre(actions ...HookFunc) TransitionBuilder {
	b.rule.preHooks = append(b.rule.preHooks, actions...)
	return b
}

func (b *builder) Post(actions ...HookFunc) TransitionBuilder {
	b.rule.postHooks = append(b.rule.postHooks, actions...)
	return b
}

func (f *fsm) Fire(ctx context.Context, event Event, currentState State, entity any) (State, error) {
	f.mu.RLock()
	rule, exists := f.rules[event]
	f.mu.RUnlock()

	if !exists {
		return currentState, fmt.Errorf("statemachinew: unknown event '%s'", event)
	}

	// 1. Validate if the transition is allowed from the current state
	if !rule.fromStates[currentState] {
		return currentState, fmt.Errorf("%w: cannot trigger '%s' from '%s'", ErrInvalidTransition, event, currentState)
	}

	// 2. Execute Guards (Validations)
	for _, guard := range rule.guards {
		if err := guard(ctx, entity); err != nil {
			return currentState, fmt.Errorf("%w: %v", ErrTransitionAborted, err)
		}
	}

	// 3. Execute Pre-Actions
	for _, pre := range rule.preHooks {
		if err := pre(ctx, entity); err != nil {
			return currentState, fmt.Errorf("statemachinew: pre-action failed: %w", err)
		}
	}

	// 4. State officially changes
	newState := rule.toState

	// 5. Execute Post-Actions
	for _, post := range rule.postHooks {
		if err := post(ctx, entity); err != nil {
			// Note: If a post-action fails, the state technically transitioned,
			// but the surrounding Usecase should handle this error (e.g., DB rollback).
			return newState, fmt.Errorf("statemachinew: post-action failed: %w", err)
		}
	}

	return newState, nil
}
