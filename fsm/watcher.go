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
	"github.com/antonf/minicloud/db"
	"github.com/oklog/ulid"
	"strings"
	"sync"
)

var (
	entityGetters = map[string]func(context.Context, db.Connection, ulid.ULID) (db.Entity, error){
		"project": func(ctx context.Context, conn db.Connection, id ulid.ULID) (db.Entity, error) {
			return conn.Projects().Get(ctx, id)
		},
		"image": func(ctx context.Context, conn db.Connection, id ulid.ULID) (db.Entity, error) {
			return conn.Images().Get(ctx, id)
		},
		"disk": func(ctx context.Context, conn db.Connection, id ulid.ULID) (db.Entity, error) {
			return conn.Disks().Get(ctx, id)
		},
	}
	machines = map[string]*StateMachine{
		"disk": DiskFSM,
	}
	prefix = "/minicloud/db/meta/notify-fsm/"
)

func WatchNotifications(ctx context.Context, conn db.Connection) error {
	notifyCh := conn.RawWatchPrefix(ctx, prefix)
	notifications, err := conn.RawReadPrefix(ctx, prefix)
	if err != nil {
		logger.Error(ctx, "failed to read notifications", "error", err)
		return err
	}
	watcher := &watcher{
		initial:  notifications,
		notifyCh: notifyCh,
		interest: make(map[string]bool),
	}
	worker := &worker{
		conn:     conn,
		workerCh: make(chan string),
	}
	go watcher.watch(ctx, worker)
	go worker.work(ctx)
	return nil
}

type watcher struct {
	initial  []db.RawValue
	notifyCh chan *db.RawValue
	minRev   int64
	interest map[string]bool
}

func (w *watcher) watch(ctx context.Context, wrk *worker) {
	for i := range w.initial {
		w.handleRawValue(ctx, wrk, &w.initial[i], true)
	}
	for {
		select {
		case rv := <-w.notifyCh:
			if rv == nil {
				logger.Info(ctx, "stopping to watch state change notifications")
				return
			}
			w.handleRawValue(ctx, wrk, rv, false)
		}
	}
}

func (w *watcher) handleRawValue(ctx context.Context, wrk *worker, rv *db.RawValue, force bool) {
	if !force && rv.ModifyRev < w.minRev {
		logger.Notice(ctx, "skipping stale notification",
			"key", rv.Key, "rev", rv.ModifyRev, "min_rev", w.minRev)
		return
	}
	if rv.ModifyRev > w.minRev {
		w.minRev = rv.ModifyRev
	}

	if strings.HasSuffix(rv.Key, "/lock") {
		// Handle notification lock
		if rv.Data != nil {
			// Lock acquired
			logger.Debug(ctx, "got lock", "key", rv.Key)
			wrk.remove(rv.Key)
		} else {
			// Lock released
			logger.Debug(ctx, "released lock", "key", rv.Key)
			key := rv.Key[:len(rv.Key)-5]
			if w.interest[key] {
				wrk.enqueue(key)
			}
		}
	} else {
		// Handle notification
		if rv.Data != nil {
			// Notification added
			w.interest[rv.Key] = true
			wrk.enqueue(rv.Key)
		} else {
			// Notification removed
			wrk.remove(rv.Key)
			delete(w.interest, rv.Key)
		}
	}
}

type worker struct {
	sync.Mutex
	conn     db.Connection
	unlocked []string
	workerCh chan string
}

func (wrk *worker) remove(key string) {
	wrk.Lock()
	defer wrk.Unlock()
	for idx, unlockedKey := range wrk.unlocked {
		if key == unlockedKey {
			unlockedLen := len(wrk.unlocked)
			wrk.unlocked[idx] = wrk.unlocked[unlockedLen-1]
			wrk.unlocked = wrk.unlocked[:unlockedLen-1]
			return
		}
	}
}

func (wrk *worker) enqueue(key string) {
	select {
	case wrk.workerCh <- key:
		return
	default:
		wrk.Lock()
		defer wrk.Unlock()
		wrk.unlocked = append(wrk.unlocked, key)
	}
}

func (wrk *worker) work(ctx context.Context) {
loop:
	for {
		// Get next key to work on
		var key string
		wrk.Lock()
		unlockedLen := len(wrk.unlocked)
		if unlockedLen > 0 {
			select {
			case <-ctx.Done():
				break loop
			default:
				key = wrk.unlocked[unlockedLen-1]
				wrk.unlocked = wrk.unlocked[:unlockedLen-1]
			}
		}
		wrk.Unlock()

		if unlockedLen == 0 {
			select {
			case <-ctx.Done():
				break loop
			case key = <-wrk.workerCh:
			}
		}

		// Work on state transfer
		wrk.processKey(ctx, key)
	}
	logger.Info(ctx, "stopped working on state transitions")
}

func (wrk *worker) processKey(ctx context.Context, key string) {
	// Get entity name and id from key
	elements := strings.Split(key[len(prefix):], "/")
	if len(elements) != 2 {
		logger.Error(ctx, "invalid notification key", "key", key)
		return
	}
	entityName := elements[0]
	id, err := ulid.Parse(elements[1])
	if err != nil {
		logger.Error(ctx, "failed to parse entity id",
			"entity", elements[0],
			"id", elements[1],
			"error", err)
		return
	}

	if getter, ok := entityGetters[entityName]; ok {
		if !wrk.lock(ctx, key) {
			return
		}
		defer wrk.unlock(ctx, key)
		entity, err := getter(ctx, wrk.conn, id)
		if err != nil {
			logger.Error(ctx, "failed to get entity",
				"entity_name", entityName,
				"id", id,
				"error", err)
			return
		}
		if stateMachine := machines[entityName]; stateMachine != nil {
			stateMachine.InvokeHook(ctx, wrk.conn, entity)
		} else {
			logger.Error(ctx, "no machine for entity", "entity_name", entityName)
		}
	}
}

// acquire lock in etcd
func (wrk *worker) lock(ctx context.Context, key string) bool {
	tx := wrk.conn.NewTransaction()
	tx.AcquireLock(ctx, key+"/lock")
	if err := tx.Commit(ctx); err != nil {
		return false
	}
	return true
}

// release lock in etcd, crash process on release failure
func (wrk *worker) unlock(ctx context.Context, key string) {
	tx := wrk.conn.NewTransaction()
	tx.ReleaseLock(ctx, key+"/lock")
	if err := tx.Commit(ctx); err != nil {
		logger.Fatal(ctx, "failed to release lock", "key", key+"/lock")
	}
}
