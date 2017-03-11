/*
 * This file is part of the MiniCloud project.
 * Copyright (C) 2017 Anton Frolov <frolov.anton@gmail.com>
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as
 * published by the Free Software Foundation, either version 3 of the
 * License, or (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */
package fsm

type State string

type StateMachine struct {
	states      []State
	initial     []State
	transitions map[State]map[State]bool
}

func uniqueAppendState(states []State, state State) []State {
	for _, s := range states {
		if s == state {
			return states
		}
	}
	return append(states, state)
}

func (sm *StateMachine) AddState(state State) *StateMachine {
	sm.states = uniqueAppendState(sm.states, state)
	return sm
}

func (sm *StateMachine) AddInitialState(state State) *StateMachine {
	sm.AddState(state)
	sm.states = uniqueAppendState(sm.initial, state)
	return sm
}

func (sm *StateMachine) AddTransition(from, to State) *StateMachine {
	sm.AddState(from)
	sm.AddState(to)
	if sm.transitions == nil {
		sm.transitions = make(map[State]map[State]bool)
	}
	trans := sm.transitions[from]
	if trans == nil {
		trans = make(map[State]bool)
		sm.transitions[from] = trans
	}
	trans[to] = true
	return sm
}

func (sm *StateMachine) CheckState(state State) error {
	for _, s := range sm.states {
		if s == state {
			return nil
		}
	}
	return &InvalidStateError{State: state}
}

func (sm *StateMachine) CheckInitialState(state State) error {
	for _, s := range sm.initial {
		if s == state {
			return nil
		}
	}
	return &InvalidStateError{State: state}
}

func (sm *StateMachine) CheckTransition(from, to State) error {
	trans := sm.transitions[from]
	if trans != nil {
		if trans[to] {
			return nil
		}
	}
	return &InvalidTransitionError{From: from, To: to}
}

func NewStateMachine(transitions ...State) *StateMachine {
	transLen := len(transitions)
	if transLen%2 != 0 {
		panic("Pairs of transitions should be passed to state machine!")
	}
	sm := &StateMachine{}
	for i := 0; i < transLen; i += 2 {
		sm.AddTransition(transitions[i], transitions[i+1])
	}
	return sm
}
