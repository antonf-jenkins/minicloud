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
package model

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/antonf/minicloud/db"
	"github.com/antonf/minicloud/utils"
	"github.com/oklog/ulid"
	"regexp"
)

type FlavorManager struct {
	conn db.Connection
}

func Flavors(conn db.Connection) *FlavorManager {
	return &FlavorManager{conn: conn}
}

var regexpFlavorName = regexp.MustCompile("^[a-z0-9_.-]{3,}$")

func (m *FlavorManager) NewEntity() *Flavor {
	return &Flavor{EntityHeader: db.EntityHeader{SchemaVersion: 1, State: db.StateCreated}}
}
func (m *FlavorManager) List(ctx context.Context) ([]*Flavor, error) {
	values, err := m.conn.RawReadPrefix(ctx, "/minicloud/db/data/flavor/")
	if err != nil {
		return nil, err
	}
	result := make([]*Flavor, len(values))
	for i, value := range values {
		entity := &Flavor{}
		origEntity := &Flavor{}
		if err := json.Unmarshal(value.Data, entity); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(value.Data, origEntity); err != nil {
			return nil, err
		}
		entity.CreateRev = value.CreateRev
		entity.ModifyRev = value.ModifyRev
		entity.Original = origEntity
		result[i] = entity
	}
	return result, nil
}
func (m *FlavorManager) Get(ctx context.Context, id ulid.ULID) (*Flavor, error) {
	value, err := m.conn.RawRead(ctx, fmt.Sprintf("/minicloud/db/data/flavor/%s", id))
	if err != nil {
		return nil, err
	}
	if value.Data == nil {
		return nil, &db.NotFoundError{Entity: "Flavor", Id: id}
	}
	entity := &Flavor{}
	if err := json.Unmarshal(value.Data, entity); err != nil {
		return nil, err
	}
	entity.CreateRev = value.CreateRev
	entity.ModifyRev = value.ModifyRev
	entity.Original = entity.Copy()
	return entity, nil
}
func (m *FlavorManager) Create(ctx context.Context, entity *Flavor, initiator db.Initiator) error {
	entity.Id = utils.NewULID()
	if !regexpFlavorName.MatchString(entity.Name) {
		return &db.FieldError{Entity: "flavor", Field: "Name", Message: "Flavor name can only consist of lowercase letters 'a' to 'z', digits, dot, dash or underscore."}
	}
	if len(entity.ServerIds) != 0 {
		return &db.FieldError{Entity: "flavor", Field: "ServerIds", Message: "Should be empty"}
	}
	txn := m.conn.NewTransaction()
	txn.Create(ctx, entity)
	key0 := fmt.Sprintf("/minicloud/db/meta/flavor/name/%s", entity.Name)
	txn.CreateMeta(ctx, key0, entity.Id.String())
	return txn.Commit(ctx)
}
func (m *FlavorManager) Update(ctx context.Context, entity *Flavor, initiator db.Initiator) error {
	origEntity := entity.Original.(*Flavor)
	if !regexpFlavorName.MatchString(entity.Name) {
		return &db.FieldError{Entity: "flavor", Field: "Name", Message: "Flavor name can only consist of lowercase letters 'a' to 'z', digits, dot, dash or underscore."}
	}
	if !utils.ULIDSliceEqual(entity.ServerIds, origEntity.ServerIds) {
		return &db.FieldError{Entity: "flavor", Field: "ServerIds", Message: "Field change prohibited"}
	}
	txn := m.conn.NewTransaction()
	txn.Update(ctx, entity)
	if entity.Name != origEntity.Name {
		forfeitKey0 := fmt.Sprintf("/minicloud/db/meta/flavor/name/%s", origEntity.Name)
		txn.CheckMeta(ctx, forfeitKey0, origEntity.Id.String())
		txn.DeleteMeta(ctx, forfeitKey0)
		claimKey0 := fmt.Sprintf("/minicloud/db/meta/flavor/name/%s", entity.Name)
		txn.CreateMeta(ctx, claimKey0, entity.Id.String())
	}
	return txn.Commit(ctx)
}
func (m *FlavorManager) Delete(ctx context.Context, id ulid.ULID, initiator db.Initiator) error {
	entity, err := Flavors(m.conn).Get(ctx, id)
	if err != nil {
		return err
	}
	if len(entity.ServerIds) != 0 {
		return &db.FieldError{Entity: "flavor", Field: "ServerIds", Message: "Should be empty"}
	}
	txn := m.conn.NewTransaction()
	txn.Delete(ctx, entity)
	key0 := fmt.Sprintf("/minicloud/db/meta/flavor/name/%s", entity.Name)
	txn.CheckMeta(ctx, key0, entity.Id.String())
	txn.DeleteMeta(ctx, key0)
	return txn.Commit(ctx)
}
