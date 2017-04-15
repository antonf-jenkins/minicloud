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
package dbimpl

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/antonf/minicloud/db"
	"github.com/antonf/minicloud/log"
	"github.com/antonf/minicloud/utils"
	backend "github.com/coreos/etcd/clientv3"
	"regexp"
	"strings"
)

func dataKey(entity db.Entity) string {
	return fmt.Sprintf("%s/%s/%s", db.DataPrefix, db.GetEntityName(entity), entity.Header().Id)
}

func (c *etcdConnection) loadEntity(ctx context.Context, entity db.Entity) error {
	entityName := db.GetEntityName(entity)
	entityId := entity.Header().Id
	key := dataKey(entity)
	opCtx := log.WithValues(ctx, "entity", entity, "key", key)

	logger.Debug(opCtx, "loading entity")
	resp, err := c.client.Get(ctx, key, backend.WithSerializable())
	if err != nil {
		logger.Error(opCtx, "etcd get failed", "error", err)
		return err
	}
	if resp.Count == 0 {
		logger.Info(opCtx, "entity not found")
		return &db.NotFoundError{Entity: entityName, Id: entityId}
	}
	kv := resp.Kvs[0]
	if err = json.Unmarshal(kv.Value, entity); err != nil {
		logger.Error(ctx, "unmarshal failed", "option", "error", err, "data", string(kv.Value))
		return err
	}
	hdr := entity.Header()
	hdr.CreateRev = kv.CreateRevision
	hdr.ModifyRev = kv.ModRevision
	hdr.Original = utils.MakeStructCopy(entity).(db.Entity)
	return nil
}

func checkFieldRegexp(entity, field, value string, regexp *regexp.Regexp) error {
	if !regexp.MatchString(value) {
		return &db.FieldError{
			Entity:  entity,
			Field:   field,
			Message: fmt.Sprintf("Field must match regexp: %s", regexp),
		}
	}
	return nil
}

func uniqueMetaKey(entity db.Entity, spec ...string) string {
	return fmt.Sprintf("%s/%s/%s", db.MetaPrefix, db.GetEntityName(entity), strings.Join(spec, "/"))
}
