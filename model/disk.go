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
)

type DiskManager struct {
	conn db.Connection
}

func Disks(conn db.Connection) *DiskManager {
	return &DiskManager{conn: conn}
}
func (m *DiskManager) NewEntity() *Disk {
	return &Disk{EntityHeader: db.EntityHeader{SchemaVersion: 1, State: db.StateCreated}}
}
func (m *DiskManager) Get(ctx context.Context, id ulid.ULID) (*Disk, error) {
	value, err := m.conn.RawRead(ctx, fmt.Sprintf("/minicloud/db/data/disk/%s", id))
	if err != nil {
		return nil, err
	}
	if value.Data == nil {
		return nil, &db.NotFoundError{Entity: "Disk", Id: id}
	}
	entity := &Disk{}
	if err := json.Unmarshal(value.Data, entity); err != nil {
		return nil, err
	}
	entity.CreateRev = value.CreateRev
	entity.ModifyRev = value.ModifyRev
	entity.Original = entity.Copy()
	return entity, nil
}
func (m *DiskManager) Create(ctx context.Context, entity *Disk, initiator db.Initiator) error {
	entity.Id = utils.NewULID()
	if err := DiskFSM.CheckInitialState(entity.State); err != nil {
		return err
	}
	if entity.ServerId != utils.Zero {
		return &db.FieldError{Entity: "disk", Field: "ServerId", Message: "Should be empty"}
	}
	txn := m.conn.NewTransaction()
	txn.Create(ctx, entity)
	if project, err := Projects(m.conn).Get(ctx, entity.ProjectId); err != nil {
		return err
	} else {
		project.DiskIds = append(project.DiskIds, entity.Id)
		txn.Update(ctx, project)
	}
	if image, err := Images(m.conn).Get(ctx, entity.ImageId); err != nil {
		return err
	} else {
		image.DiskIds = append(image.DiskIds, entity.Id)
		txn.Update(ctx, image)
	}
	DiskFSM.Notify(ctx, txn, entity)
	return txn.Commit(ctx)
}
func (m *DiskManager) Update(ctx context.Context, entity *Disk, initiator db.Initiator) error {
	origEntity := entity.Original.(*Disk)
	if err := DiskFSM.CheckTransition(origEntity.State, entity.State, initiator); err != nil {
		return err
	}
	if entity.ProjectId != origEntity.ProjectId {
		return &db.FieldError{Entity: "disk", Field: "ProjectId", Message: "Field change prohibited"}
	}
	if entity.ImageId != origEntity.ImageId {
		return &db.FieldError{Entity: "disk", Field: "ImageId", Message: "Field change prohibited"}
	}
	if entity.ServerId != origEntity.ServerId {
		return &db.FieldError{Entity: "disk", Field: "ServerId", Message: "Field change prohibited"}
	}
	txn := m.conn.NewTransaction()
	txn.Update(ctx, entity)
	DiskFSM.Notify(ctx, txn, entity)
	return txn.Commit(ctx)
}
func (m *DiskManager) IntentDelete(ctx context.Context, id ulid.ULID, initiator db.Initiator) error {
	entity, err := Disks(m.conn).Get(ctx, id)
	if err != nil {
		return err
	}
	if err := DiskFSM.CheckTransition(entity.State, db.StateDeleting, initiator); err != nil {
		return err
	}
	if entity.ServerId != utils.Zero {
		return &db.FieldError{Entity: "disk", Field: "ServerId", Message: "Should be empty"}
	}
	entity.State = db.StateDeleting
	txn := m.conn.NewTransaction()
	txn.Update(ctx, entity)
	DiskFSM.Notify(ctx, txn, entity)
	return txn.Commit(ctx)
}
func (m *DiskManager) Delete(ctx context.Context, id ulid.ULID, initiator db.Initiator) error {
	entity, err := Disks(m.conn).Get(ctx, id)
	if err != nil {
		return err
	}
	if err := DiskFSM.CheckTransition(entity.State, db.StateDeleted, initiator); err != nil {
		return err
	}
	if entity.ServerId != utils.Zero {
		return &db.FieldError{Entity: "disk", Field: "ServerId", Message: "Should be empty"}
	}
	txn := m.conn.NewTransaction()
	txn.Delete(ctx, entity)
	if project, err := Projects(m.conn).Get(ctx, entity.ProjectId); err != nil {
		return err
	} else {
		project.DiskIds = utils.RemoveULID(project.DiskIds, entity.Id)
		txn.Update(ctx, project)
	}
	if image, err := Images(m.conn).Get(ctx, entity.ImageId); err != nil {
		return err
	} else {
		image.DiskIds = utils.RemoveULID(image.DiskIds, entity.Id)
		txn.Update(ctx, image)
	}
	DiskFSM.DeleteNotification(ctx, txn, entity)
	return txn.Commit(ctx)
}
