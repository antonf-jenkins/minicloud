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
type Initiator int

const (
	System Initiator = 1 << 0
	User   Initiator = 1 << 1
)

type StateMachine struct {
	states      []State
	initial     []State
	transitions map[State]map[State]Initiator
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

func (sm *StateMachine) InitialState(state State) *StateMachine {
	sm.AddState(state)
	sm.states = uniqueAppendState(sm.initial, state)
	return sm
}

func (sm *StateMachine) addTransition(from, to State, initiator Initiator) *StateMachine {
	sm.AddState(from)
	sm.AddState(to)
	if sm.transitions == nil {
		sm.transitions = make(map[State]map[State]Initiator)
	}
	trans := sm.transitions[from]
	if trans == nil {
		trans = make(map[State]Initiator)
		sm.transitions[from] = trans
	}
	trans[to] = initiator
	return sm
}

func (sm *StateMachine) Transition(from, to State) *StateMachine {
	return sm.addTransition(from, to, System|User)
}

func (sm *StateMachine) SystemTransition(from, to State) *StateMachine {
	return sm.addTransition(from, to, System)
}

func (sm *StateMachine) UserTransition(from, to State) *StateMachine {
	return sm.addTransition(from, to, User)
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

func (sm *StateMachine) CheckTransition(from, to State, initiator Initiator) error {
	trans := sm.transitions[from]
	if trans != nil && (trans[to]&initiator) != 0 {
		return nil
	}
	return &InvalidTransitionError{From: from, To: to}
}

func NewStateMachine() *StateMachine {
	sm := &StateMachine{}
	return sm
}
