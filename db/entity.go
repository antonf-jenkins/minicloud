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
	"reflect"
	"strings"
)

const (
	MetaPrefix = "/minicloud/db/meta"
	DataPrefix = "/minicloud/db/data"
)

type Entity interface {
	createRev() int64
	setCreateRev(rev int64)
	modifyRev() int64
	setModifyRev(rev int64)
	setOriginal(entity Entity)
}

type EntityHeader struct {
	SchemaVersion int64
	CreateRev     int64  `json:"-"`
	ModifyRev     int64  `json:"-"`
	original      Entity `json:"-"`
}

// Method returning create revision to satisfy Entity interface
func (hdr *EntityHeader) createRev() int64 {
	return hdr.CreateRev
}

// Method setting create revision to satisfy Entity interface
func (hdr *EntityHeader) setCreateRev(rev int64) {
	hdr.CreateRev = rev
}

// Method returning modify revision to satisfy Entity interface
func (hdr *EntityHeader) modifyRev() int64 {
	return hdr.ModifyRev
}

// Method setting modify revision to satisfy Entity interface
func (hdr *EntityHeader) setModifyRev(rev int64) {
	hdr.ModifyRev = rev
}

func (hdr *EntityHeader) setOriginal(entity Entity) {
	hdr.original = entity
}

func GetEntityId(entity Entity) ulid.ULID {
	entityRv := reflect.ValueOf(entity)
	if entityRv.Kind() == reflect.Ptr {
		entityRv = entityRv.Elem()
	}
	return entityRv.FieldByName("Id").Interface().(ulid.ULID)
}

func GetEntityName(entity Entity) string {
	ty := reflect.TypeOf(entity)
	if ty.Kind() == reflect.Ptr {
		ty = ty.Elem()
	}
	return strings.ToLower(ty.Name())
}

func dataKey(entity Entity) string {
	return fmt.Sprintf("%s/%s/%s", DataPrefix, GetEntityName(entity), GetEntityId(entity))
}

func (db *etcdConeection) loadEntity(ctx context.Context, entity Entity) error {
	entityName := GetEntityName(entity)
	entityId := GetEntityId(entity)
	key := dataKey(entity)
	log.Printf("db: loading entity=%s id=%s key=%s", entityName, entityId, key)
	resp, err := db.client.Get(ctx, key, backend.WithSerializable())
	if err != nil {
		log.Printf("db: get %s failed: %s", key, err)
		return err
	}
	if resp.Count == 0 {
		log.Printf("db: entity not found entityName=%s id=%s", entityName, entityId)
		return &NotFoundError{entityName, entityId}
	}
	kv := resp.Kvs[0]
	if err = json.Unmarshal(kv.Value, entity); err != nil {
		log.Printf("db: unmarshal failed data='%s': %s", kv.Value, err)
		return err
	}
	entity.setCreateRev(kv.CreateRevision)
	entity.setModifyRev(kv.ModRevision)
	entity.setOriginal(utils.MakeStructCopy(entity).(Entity))
	return nil
}
