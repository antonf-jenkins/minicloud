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

import (
	"context"
	"fmt"
	"github.com/antonf/minicloud/db"
	"github.com/antonf/minicloud/utils"
	"github.com/oklog/ulid"
)

type Hook func(ctx context.Context, conn db.Connection, entity db.Entity)

// TODO: check that there is no transition possible between hooked states
// e.g Created is hooked and InUse is hooked => make sure that there is no
// transition between Created and InUse
type StateMachine struct {
	states      []db.State
	initial     []db.State
	transitions map[db.State]map[db.State]db.Initiator
	hooks       map[db.State]Hook
}

func NewStateMachine() *StateMachine {
	sm := &StateMachine{
		transitions: make(map[db.State]map[db.State]db.Initiator),
		hooks:       make(map[db.State]Hook),
	}
	return sm
}

func uniqueAppendState(states []db.State, state db.State) []db.State {
	for _, s := range states {
		if s == state {
			return states
		}
	}
	return append(states, state)
}

func (sm *StateMachine) AddState(state db.State) *StateMachine {
	sm.states = uniqueAppendState(sm.states, state)
	return sm
}

func (sm *StateMachine) InitialState(state db.State) *StateMachine {
	sm.AddState(state)
	sm.initial = uniqueAppendState(sm.initial, state)
	return sm
}

func (sm *StateMachine) addTransition(from, to db.State, initiator db.Initiator) *StateMachine {
	sm.AddState(from)
	sm.AddState(to)
	trans := sm.transitions[from]
	if trans == nil {
		trans = make(map[db.State]db.Initiator)
		sm.transitions[from] = trans
	}
	trans[to] = initiator
	return sm
}

func (sm *StateMachine) Transition(from, to db.State) *StateMachine {
	return sm.addTransition(from, to, db.InitiatorSystem|db.InitiatorUser)
}

func (sm *StateMachine) SystemTransition(from, to db.State) *StateMachine {
	return sm.addTransition(from, to, db.InitiatorSystem)
}

func (sm *StateMachine) UserTransition(from, to db.State) *StateMachine {
	return sm.addTransition(from, to, db.InitiatorUser)
}

func (sm *StateMachine) CheckState(state db.State) error {
	for _, s := range sm.states {
		if s == state {
			return nil
		}
	}
	return &InvalidStateError{State: state}
}

func (sm *StateMachine) CheckInitialState(state db.State) error {
	for _, s := range sm.initial {
		if s == state {
			return nil
		}
	}
	return &InvalidStateError{State: state}
}

func (sm *StateMachine) CheckTransition(from, to db.State, initiator db.Initiator) error {
	trans := sm.transitions[from]
	if trans != nil && (trans[to]&initiator) != 0 {
		return nil
	}
	return &InvalidTransitionError{From: from, To: to}
}

func (fsm *StateMachine) Hook(state db.State, handler Hook) *StateMachine {
	fsm.hooks[state] = handler
	return fsm
}

func (fsm *StateMachine) InvokeHook(ctx context.Context, conn db.Connection, entity db.Entity) {
	hook := fsm.hooks[entity.Header().State]
	if hook != nil {
		hook(ctx, conn, entity)
	}
}

func (fsm *StateMachine) NeedNotify(state db.State) bool {
	return fsm.hooks[state] != nil
}

func notificationKey(entityName string, id ulid.ULID, state db.State) string {
	return fmt.Sprintf("%s%s/%s/%s", prefix, entityName, id.String(), state)
}

func (fsm *StateMachine) Notify(ctx context.Context, tx db.Transaction, entity db.Entity) {
	hdr := entity.Header()
	toState := hdr.State
	fromState := db.StateNone
	if hdr.Original != nil {
		fromState = hdr.Original.Header().State
	}
	if toState == fromState {
		return
	}
	entityName := db.GetEntityName(entity)
	if fsm.NeedNotify(fromState) {
		// Delete previous notification
		tx.DeleteMeta(ctx, notificationKey(entityName, hdr.Id, fromState))
	}
	if fsm.NeedNotify(toState) {
		// Create notification
		notificationId := utils.NewULID()
		logger.Debug(ctx, "creating new notification",
			"entity", entityName,
			"state", toState,
			"notification_id", notificationId)
		tx.CreateMeta(ctx, notificationKey(entityName, hdr.Id, toState), notificationId.String())
	}
}

func (fsm *StateMachine) DeleteNotification(ctx context.Context, tx db.Transaction, entity db.Entity) {
	hdr := entity.Header()
	if fsm.NeedNotify(hdr.State) {
		entityName := db.GetEntityName(entity)
		tx.DeleteMeta(ctx, notificationKey(entityName, hdr.Id, hdr.State))
	}
}
