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
	"github.com/antonf/minicloud/utils"
	"github.com/oklog/ulid"
	"strings"
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

func splitEntityAndId(ctx context.Context, key string) (string, ulid.ULID) {
	elements := strings.Split(key[len(prefix):], "/")
	if len(elements) != 2 {
		logger.Error(ctx, "invalid notification key", "key", key)
		return "", utils.Zero
	}
	if id, err := ulid.Parse(elements[1]); err != nil {
		logger.Error(ctx, "failed to parse entity id", "entity", elements[0], "id", elements[1], "error", err)
		return "", utils.Zero
	} else {
		return elements[0], id
	}
}

func handleNotification(ctx context.Context, conn db.Connection, rv *db.RawValue) {
	entityName, id := splitEntityAndId(ctx, rv.Key)
	if getter, ok := entityGetters[entityName]; ok {
		entity, err := getter(ctx, conn, id)
		if err != nil {
			logger.Error(ctx, "failed to get entity", "entity_name", entityName, "id", id, "error", err)
			return
		}
		if fsm := machines[entityName]; fsm != nil {
			fsm.InvokeHook(ctx, conn, entity)
		} else {
			logger.Error(ctx, "no machine for entity", "entity_name", entityName)
		}
	}
}

func WatchNotifications(ctx context.Context, conn db.Connection) {
	notifyCh := conn.RawWatchPrefix(ctx, prefix)
	go func() {
		var minRev int64
		notifications, err := conn.RawReadPrefix(ctx, prefix)
		if err != nil {
			logger.Error(ctx, "failed to read notifications", "error", err)
		}
		for _, rv := range notifications {
			handleNotification(ctx, conn, &rv)
			if rv.ModifyRev > minRev {
				minRev = rv.ModifyRev
			}
		}
		for {
			select {
			case rv := <-notifyCh:
				if rv == nil {
					return
				}
				if rv.ModifyRev > minRev {
					handleNotification(ctx, conn, rv)
					minRev = rv.ModifyRev
				} else {
					logger.Notice(ctx, "skipping stale notification",
						"key", rv.Key, "rev", rv.ModifyRev, "min_rev", minRev)
				}
			case <-ctx.Done():
				return
			}
		}
	}()
}
