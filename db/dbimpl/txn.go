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
	"strconv"
	"strings"
)

func dataKey(entity db.Entity) string {
	return fmt.Sprintf("/minicloud/db/data/%s/%s", strings.ToLower(entity.EntityName()), entity.Header().Id)
}

func (c *etcdConnection) NewTransaction() db.Transaction {
	return &etcdTransaction{
		xid:  utils.NewULID().String(),
		conn: c,
	}
}

type etcdTransaction struct {
	xid  string
	err  error
	conn *etcdConnection
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
	logger.Debug(ctx, "commiting transaction", "xid", t.xid)
	txn := t.conn.client.KV.Txn(ctx)
	resp, err := txn.If(t.cmps...).Then(t.ops...).Commit()
	if err != nil {
		logger.Error(ctx, "error commiting transaction", "xid", t.xid, "error", err)
		return err
	}
	if !resp.Succeeded {
		return &db.ConflictError{Xid: t.xid}
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

func (t *etcdTransaction) CreateMeta(ctx context.Context, key string, content string) {
	if t.err != nil {
		return
	}
	logger.Debug(ctx, "create meta", "key", key, "content", content, "xid", t.xid)
	t.addCmp(backend.Version(key), "=", 0)
	t.addOp(backend.OpPut(key, content))
}

func (t *etcdTransaction) DeleteMeta(ctx context.Context, key string) {
	if t.err != nil {
		return
	}
	logger.Debug(ctx, "delete meta", "key", key, "xid", t.xid)
	t.addOp(backend.OpDelete(key))
}

func (t *etcdTransaction) CheckMeta(ctx context.Context, key, content string) {
	if t.err != nil {
		return
	}
	logger.Debug(ctx, "check meta content", "key", key, "content", content, "xid", t.xid)
	t.addCmp(backend.Value(key), "=", content)
}

func (t *etcdTransaction) AcquireLock(ctx context.Context, key string) {
	leaseId := t.conn.leaseId
	content := strconv.FormatInt(int64(leaseId), 16)
	logger.Debug(ctx, "acquiring lock", "key", key, "lease_id", content)
	t.addCmp(backend.Version(key), "=", 0)
	t.addOp(backend.OpPut(key, content, backend.WithLease(leaseId)))
}

func (t *etcdTransaction) ReleaseLock(ctx context.Context, key string) {
	leaseId := t.conn.leaseId
	content := strconv.FormatInt(int64(leaseId), 16)
	logger.Debug(ctx, "releasing lock", "key", key, "content", content)
	t.addCmp(backend.Value(key), "=", content)
	t.addOp(backend.OpDelete(key))
}
