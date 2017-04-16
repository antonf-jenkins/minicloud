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

func buildEntity(ctx context.Context, entity db.Entity, data []byte, createRev, modifyRev int64) error {
	if err := json.Unmarshal(data, entity); err != nil {
		logger.Error(ctx, "unmarshal failed", "option", "error", err, "data", string(data))
		return err
	}
	hdr := entity.Header()
	hdr.CreateRev = createRev
	hdr.ModifyRev = modifyRev
	hdr.Original = utils.MakeStructCopy(entity).(db.Entity)
	return nil
}

func (c *etcdConnection) listEntities(ctx context.Context, entityName string, createFn func() db.Entity) ([]db.Entity, error) {
	opCtx := log.WithValues(ctx, "entity_name", entityName)
	prefix := fmt.Sprintf("%s/%s/", db.DataPrefix, entityName)

	logger.Debug(opCtx, "loading entity list")
	resp, err := c.client.Get(opCtx, prefix, backend.WithSerializable(), backend.WithPrefix())
	if err != nil {
		logger.Error(opCtx, "etcd get failed", "error", err)
		return nil, err
	}
	result := make([]db.Entity, resp.Count)
	for index, kv := range resp.Kvs {
		entity := createFn()
		if err := buildEntity(opCtx, entity, kv.Value, kv.CreateRevision, kv.ModRevision); err != nil {
			return nil, err
		}
		result[index] = entity
	}
	return result, nil
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
	return buildEntity(opCtx, entity, kv.Value, kv.CreateRevision, kv.ModRevision)
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
