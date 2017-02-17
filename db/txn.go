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
package db

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/antonf/minicloud/utils"
	backend "github.com/coreos/etcd/clientv3"
	"github.com/oklog/ulid"
	"log"
)

type Transaction interface {
	Commit(ctx context.Context) error
	Create(value Entity)
	Update(value Entity)
	Delete(value Entity)
	ForceDelete(entity string, id ulid.ULID)
	ClaimUnique(value Entity, field string)
	ForfeitUnique(entity Entity, field string)
}

func (c *etcdConeection) NewTransaction() Transaction {
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
		log.Printf("db: txn %s: previous error, aborting: %s", t.xid, t.err)
		return t.err
	}
	log.Printf("db: txn %s: committing", t.xid)
	txn := t.conn.client.KV.Txn(ctx)
	resp, err := txn.If(t.cmps...).Then(t.ops...).Commit()
	if err != nil {
		return err
	}
	if !resp.Succeeded {
		return &ConflictError{t.xid}
	}
	return nil
}

func (t *etcdTransaction) Create(entity Entity) {
	if t.err != nil {
		return
	}
	log.Printf("db: txn %s: creating entity=%s", t.xid, entity)
	marshaledEntity, err := json.Marshal(entity)
	if err != nil {
		log.Printf("db: txn %s: marshal failed: %s", t.xid, err)
		t.err = err
		return
	}
	key := dataKey(entity)
	t.addCmp(backend.Version(key), "=", 0)
	t.addOp(backend.OpPut(key, string(marshaledEntity)))
}

func (t *etcdTransaction) Update(entity Entity) {
	if t.err != nil {
		return
	}
	log.Printf("db: txn %s: updating entity=%s", t.xid, entity)
	marshaledEntity, err := json.Marshal(entity)
	if err != nil {
		log.Printf("db: txn %s: marshal failed: %s", t.xid, err)
		t.err = err
		return
	}
	key := dataKey(entity)
	t.addCmp(backend.Version(key), "!=", 0)
	t.addCmp(backend.ModRevision(key), "=", entity.modifyRev())
	t.addOp(backend.OpPut(key, string(marshaledEntity)))
}

func (t *etcdTransaction) Delete(entity Entity) {
	if t.err != nil {
		return
	}
	log.Printf("db: txn %s: deleting entity=%s", t.xid, entity)
	key := dataKey(entity)
	t.addCmp(backend.ModRevision(key), "=", entity.modifyRev())
	t.addOp(backend.OpDelete(key))
}

func (t *etcdTransaction) ForceDelete(entityName string, id ulid.ULID) {
	if t.err != nil {
		return
	}
	log.Printf("db: txn %s: deleting entityName=%s id=%s", t.xid, entityName, id)
	key := fmt.Sprintf("%s/%s/%s", DataPrefix, entityName, id)
	t.addOp(backend.OpDelete(key))
}

func (t *etcdTransaction) ClaimUnique(entity Entity, field string) {
	if t.err != nil {
		return
	}
	entityName := getEntityName(entity)
	fieldValue := utils.GetFieldValue(entity, field)
	log.Printf("db: txn %s: claim unique entity=%s field=%s value='%s'", t.xid, entityName, field, fieldValue)
	key := fmt.Sprintf("%s/%s/%s/%s", MetaPrefix, entityName, field, fieldValue)
	entityId := getEntityId(entity).String()
	t.addCmp(backend.Version(key), "=", 0)
	t.addOp(backend.OpPut(key, entityId))
}

func (t *etcdTransaction) ForfeitUnique(entity Entity, field string) {
	if t.err != nil {
		return
	}
	entityName := getEntityName(entity)
	fieldValue := utils.GetFieldValue(entity, field)
	log.Printf("db: txn %s: forfeit unique entity=%s field=%s value='%s'", t.xid, entityName, field, fieldValue)
	key := fmt.Sprintf("%s/%s/%s/%s", MetaPrefix, entityName, field, fieldValue)
	entityId := getEntityId(entity).String()
	t.addCmp(backend.Value(key), "=", entityId)
	t.addOp(backend.OpDelete(key))
}
