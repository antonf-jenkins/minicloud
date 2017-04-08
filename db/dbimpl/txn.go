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
	"github.com/antonf/minicloud/utils"
	backend "github.com/coreos/etcd/clientv3"
	"github.com/oklog/ulid"
	"strings"
)

func (c *etcdConeection) NewTransaction() db.Transaction {
	return &etcdTransaction{
		xid:  utils.NewULID().String(),
		conn: c,
	}
}

type etcdTransaction struct {
	xid  string
	err  error
	conn *etcdConeection
	cmps []backend.Cmp
	ops  []backend.Op
}

func (t *etcdTransaction) addCmp(cmp backend.Cmp, result string, v interface{}) {
	t.cmps = append(t.cmps, backend.Compare(cmp, result, v))
}

func (t *etcdTransaction) addOp(op backend.Op) {
	t.ops = append(t.ops, op)
}

func (t *etcdTransaction) Commit(ctx context.Context) error {
	if t.err != nil {
		logger.Error(ctx, "aborting transaction", "xid", t.xid, "error", t.err)
		return t.err
	}
	logger.Error(ctx, "commiting transaction", "xid", t.xid)
	txn := t.conn.client.KV.Txn(ctx)
	resp, err := txn.If(t.cmps...).Then(t.ops...).Commit()
	if err != nil {
		logger.Error(ctx, "error commiting transaction", "xid", t.xid, "error", err)
		return err
	}
	if !resp.Succeeded {
		return &db.ConflictError{t.xid}
	}
	return nil
}

func (t *etcdTransaction) Create(ctx context.Context, entity db.Entity) {
	if t.err != nil {
		return
	}
	logger.Debug(ctx, "creating entity", "xid", t.xid, "entity", entity)
	marshaledEntity, err := json.Marshal(entity)
	if err != nil {
		logger.Debug(ctx, "marshal failed", "xid", t.xid, "entity", entity, "error", err)
		t.err = err
		return
	}
	key := dataKey(entity)
	t.addCmp(backend.Version(key), "=", 0)
	t.addOp(backend.OpPut(key, string(marshaledEntity)))
}

func (t *etcdTransaction) Update(ctx context.Context, entity db.Entity) {
	if t.err != nil {
		return
	}
	logger.Debug(ctx, "updating entity", "xid", t.xid, "entity", entity)
	marshaledEntity, err := json.Marshal(entity)
	if err != nil {
		logger.Debug(ctx, "marshal failed", "xid", t.xid, "entity", entity, "error", err)
		t.err = err
		return
	}
	key := dataKey(entity)
	t.addCmp(backend.Version(key), "!=", 0)
	t.addCmp(backend.ModRevision(key), "=", entity.Header().ModifyRev)
	t.addOp(backend.OpPut(key, string(marshaledEntity)))
}

func (t *etcdTransaction) Delete(ctx context.Context, entity db.Entity) {
	if t.err != nil {
		return
	}
	logger.Debug(ctx, "deleting entity", "xid", t.xid, "entity", entity)
	key := dataKey(entity)
	t.addCmp(backend.ModRevision(key), "=", entity.Header().ModifyRev)
	t.addOp(backend.OpDelete(key))
}

func (t *etcdTransaction) ForceDelete(ctx context.Context, entityName string, id ulid.ULID) {
	if t.err != nil {
		return
	}
	logger.Debug(ctx, "force deleting entity", "xid", t.xid, "entity_name", entityName, "id", id)
	key := fmt.Sprintf("%s/%s/%s", DataPrefix, entityName, id)
	t.addOp(backend.OpDelete(key))
}

func metaKey(name string, spec []string) string {
	return fmt.Sprintf("%s/%s/%s", MetaPrefix, name, strings.Join(spec, "/"))
}

func (t *etcdTransaction) ClaimUnique(ctx context.Context, entity db.Entity, spec ...string) {
	if t.err != nil {
		return
	}
	entityName := GetEntityName(entity)
	key := metaKey(entityName, spec)
	logger.Debug(ctx, "claim unique", "entity_name", entityName, "spec", spec, "xid", t.xid)
	entityId := entity.Header().Id.String()
	t.addCmp(backend.Version(key), "=", 0)
	t.addOp(backend.OpPut(key, entityId))
}

func (t *etcdTransaction) ForfeitUnique(ctx context.Context, entity db.Entity, spec ...string) {
	if t.err != nil {
		return
	}
	entityName := GetEntityName(entity)
	key := metaKey(entityName, spec)
	logger.Debug(ctx, "forfeit unique", "entity_name", entityName, "spec", spec, "xid", t.xid)
	entityId := entity.Header().Id.String()
	t.addCmp(backend.Value(key), "=", entityId)
	t.addOp(backend.OpDelete(key))
}

func (t *etcdTransaction) CreateMeta(ctx context.Context, path []string, content string) {
	if t.err != nil {
		return
	}
	key := MetaPrefix + "/" + strings.Join(path, "/")
	logger.Debug(ctx, "create meta", "path", path, "content", content, "xid", t.xid)
	t.addCmp(backend.Version(key), "=", 0)
	t.addOp(backend.OpPut(key, content))
}

func (t *etcdTransaction) DeleteMeta(ctx context.Context, path []string) {
	if t.err != nil {
		return
	}
	key := MetaPrefix + "/" + strings.Join(path, "/")
	logger.Debug(ctx, "delete meta", "path", path, "xid", t.xid)
	t.addOp(backend.OpDelete(key))
}

func (t *etcdTransaction) DeleteMetaContent(ctx context.Context, path []string, content string) {
	if t.err != nil {
		return
	}
	key := MetaPrefix + "/" + strings.Join(path, "/")
	logger.Debug(ctx, "check content and delete meta", "path", path, "content", content, "xid", t.xid)
	t.addCmp(backend.Value(key), "=", content)
	t.addOp(backend.OpDelete(key))
}
