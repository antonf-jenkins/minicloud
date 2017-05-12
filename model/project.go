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

type ProjectManager struct {
	conn db.Connection
}

func Projects(conn db.Connection) *ProjectManager {
	return &ProjectManager{conn: conn}
}

var regexpProjectName = regexp.MustCompile("^[a-zA-Z0-9_.:-]{3,200}$")

func (m *ProjectManager) NewEntity() *db.Project {
	return &db.Project{EntityHeader: db.EntityHeader{SchemaVersion: 1, State: db.StateCreated}}
}
func (m *ProjectManager) List(ctx context.Context) ([]*db.Project, error) {
	values, err := m.conn.RawReadPrefix(ctx, "/minicloud/db/data/project/")
	if err != nil {
		return nil, err
	}
	result := make([]*db.Project, len(values))
	for i, value := range values {
		entity := &db.Project{}
		origEntity := &db.Project{}
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
func (m *ProjectManager) Get(ctx context.Context, id ulid.ULID) (*db.Project, error) {
	value, err := m.conn.RawRead(ctx, fmt.Sprintf("/minicloud/db/data/project/%s", id))
	if err != nil {
		return nil, err
	}
	if value.Data == nil {
		return nil, &db.NotFoundError{Entity: "Project", Id: id}
	}
	entity := &db.Project{}
	origEntity := &db.Project{}
	if err := json.Unmarshal(value.Data, entity); err != nil {
		return nil, err
	}
	if err := json.Unmarshal(value.Data, origEntity); err != nil {
		return nil, err
	}
	entity.CreateRev = value.CreateRev
	entity.ModifyRev = value.ModifyRev
	entity.Original = origEntity
	return entity, nil
}
func (m *ProjectManager) Create(ctx context.Context, entity *db.Project, initiator db.Initiator) error {
	entity.Id = utils.NewULID()
	if !regexpProjectName.MatchString(entity.Name) {
		return &db.FieldError{Entity: "project", Field: "Name", Message: "Name should be between 3 and 200 characters from following set: a-z A-Z 0-9 _.:-"}
	}
	if len(entity.ImageIds) != 0 {
		return &db.FieldError{Entity: "project", Field: "ImageIds", Message: "Should be empty"}
	}
	if len(entity.DiskIds) != 0 {
		return &db.FieldError{Entity: "project", Field: "DiskIds", Message: "Should be empty"}
	}
	if len(entity.ServerIds) != 0 {
		return &db.FieldError{Entity: "project", Field: "ServerIds", Message: "Should be empty"}
	}
	txn := m.conn.NewTransaction()
	txn.Create(ctx, entity)
	key0 := fmt.Sprintf("/minicloud/db/meta/project/name/%s", entity.Name)
	txn.CreateMeta(ctx, key0, entity.Id.String())
	return txn.Commit(ctx)
}
func (m *ProjectManager) Update(ctx context.Context, entity *db.Project, initiator db.Initiator) error {
	origEntity := entity.Original.(*db.Project)
	if !regexpProjectName.MatchString(entity.Name) {
		return &db.FieldError{Entity: "project", Field: "Name", Message: "Name should be between 3 and 200 characters from following set: a-z A-Z 0-9 _.:-"}
	}
	if !utils.ULIDSliceEqual(entity.ImageIds, origEntity.ImageIds) {
		return &db.FieldError{Entity: "project", Field: "ImageIds", Message: "Field change prohibited"}
	}
	if !utils.ULIDSliceEqual(entity.DiskIds, origEntity.DiskIds) {
		return &db.FieldError{Entity: "project", Field: "DiskIds", Message: "Field change prohibited"}
	}
	if !utils.ULIDSliceEqual(entity.ServerIds, origEntity.ServerIds) {
		return &db.FieldError{Entity: "project", Field: "ServerIds", Message: "Field change prohibited"}
	}
	txn := m.conn.NewTransaction()
	txn.Update(ctx, entity)
	if entity.Name != origEntity.Name {
		forfeitKey0 := fmt.Sprintf("/minicloud/db/meta/project/name/%s", origEntity.Name)
		txn.CheckMeta(ctx, forfeitKey0, origEntity.Id.String())
		txn.DeleteMeta(ctx, forfeitKey0)
		claimKey0 := fmt.Sprintf("/minicloud/db/meta/project/name/%s", entity.Name)
		txn.CreateMeta(ctx, claimKey0, entity.Id.String())
	}
	return txn.Commit(ctx)
}
func (m *ProjectManager) Delete(ctx context.Context, id ulid.ULID, initiator db.Initiator) error {
	entity, err := Projects(m.conn).Get(ctx, id)
	if err != nil {
		return err
	}
	if len(entity.ImageIds) != 0 {
		return &db.FieldError{Entity: "project", Field: "ImageIds", Message: "Should be empty"}
	}
	if len(entity.DiskIds) != 0 {
		return &db.FieldError{Entity: "project", Field: "DiskIds", Message: "Should be empty"}
	}
	if len(entity.ServerIds) != 0 {
		return &db.FieldError{Entity: "project", Field: "ServerIds", Message: "Should be empty"}
	}
	txn := m.conn.NewTransaction()
	txn.Delete(ctx, entity)
	key0 := fmt.Sprintf("/minicloud/db/meta/project/name/%s", entity.Name)
	txn.CheckMeta(ctx, key0, entity.Id.String())
	txn.DeleteMeta(ctx, key0)
	return txn.Commit(ctx)
}
