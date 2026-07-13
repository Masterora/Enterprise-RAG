package agent

import "fmt"

type State string

const (
	StateInitialized  State = "initialized"
	StatePlanning     State = "planning"
	StateExecuting    State = "executing"
	StateObserving    State = "observing"
	StateSynthesizing State = "synthesizing"
	StateCompleted    State = "completed"
	StateFailed       State = "failed"
)

var stateTransitions = map[State]map[State]struct{}{
	StateInitialized:  {StatePlanning: {}, StateFailed: {}},
	StatePlanning:     {StateExecuting: {}, StateSynthesizing: {}, StateFailed: {}},
	StateExecuting:    {StateObserving: {}, StateFailed: {}},
	StateObserving:    {StatePlanning: {}, StateSynthesizing: {}, StateFailed: {}},
	StateSynthesizing: {StateCompleted: {}, StateFailed: {}},
}

type StateMachine struct {
	state     State
	iteration int
	observer  func(State, State)
}

func (m *StateMachine) SetObserver(observer func(State, State)) {
	m.observer = observer
}

func NewStateMachine() *StateMachine {
	return &StateMachine{state: StateInitialized}
}

func (m *StateMachine) State() State {
	return m.state
}

func (m *StateMachine) Iteration() int {
	return m.iteration
}

func (m *StateMachine) StartIteration() error {
	if err := m.Transition(StatePlanning); err != nil {
		return err
	}
	m.iteration++
	return nil
}

func (m *StateMachine) Transition(next State) error {
	previous := m.state
	allowed, ok := stateTransitions[m.state]
	if !ok {
		return fmt.Errorf("agent state %q is terminal", m.state)
	}
	if _, ok := allowed[next]; !ok {
		return fmt.Errorf("invalid agent state transition %q -> %q", m.state, next)
	}
	m.state = next
	if m.observer != nil {
		m.observer(previous, next)
	}
	return nil
}
