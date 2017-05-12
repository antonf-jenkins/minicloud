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

type ServerManager struct {
	conn db.Connection
}

func Servers(conn db.Connection) *ServerManager {
	return &ServerManager{conn: conn}
}

var regexpServerName = regexp.MustCompile("^[a-z]([a-z0-9-]*[a-z0-9])?$|^[0-9][a-z0-9-]*([a-z]([a-z0-9-]*[a-z0-9])?|-[a-z0-9-]*[a-z0-9])$")

func (m *ServerManager) NewEntity() *db.Server {
	return &db.Server{EntityHeader: db.EntityHeader{SchemaVersion: 1, State: db.StateCreated}}
}
func (m *ServerManager) Get(ctx context.Context, id ulid.ULID) (*db.Server, error) {
	value, err := m.conn.RawRead(ctx, fmt.Sprintf("/minicloud/db/data/server/%s", id))
	if err != nil {
		return nil, err
	}
	if value.Data == nil {
		return nil, &db.NotFoundError{Entity: "Server", Id: id}
	}
	entity := &db.Server{}
	origEntity := &db.Server{}
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
func (m *ServerManager) Create(ctx context.Context, entity *db.Server, initiator db.Initiator) error {
	entity.Id = utils.NewULID()
	if err := ServerFSM.CheckInitialState(entity.State); err != nil {
		return err
	}
	if !regexpServerName.MatchString(entity.Name) {
		return &db.FieldError{Entity: "server", Field: "Name", Message: "Should contain only lowercase letters, digits and dash, but shouldn't start or end with dash"}
	}
	txn := m.conn.NewTransaction()
	txn.Create(ctx, entity)
	if project, err := Projects(m.conn).Get(ctx, entity.ProjectId); err != nil {
		return err
	} else {
		project.ServerIds = append(project.ServerIds, entity.Id)
		txn.Update(ctx, project)
	}
	if flavor, err := Flavors(m.conn).Get(ctx, entity.FlavorId); err != nil {
		return err
	} else {
		flavor.ServerIds = append(flavor.ServerIds, entity.Id)
		txn.Update(ctx, flavor)
	}
	for _, refEntityId := range entity.DiskIds {
		if disk, err := Disks(m.conn).Get(ctx, refEntityId); err != nil {
			return err
		} else {
			if disk.ServerId != utils.Zero {
				return &db.FieldError{Entity: "Disk", Field: "ServerId", Message: "Should be empty"}
			}
			disk.ServerId = entity.Id
			if err := DiskFSM.ChangeState(disk, db.StateInUse, db.InitiatorSystem); err != nil {
				return err
			}
			txn.Update(ctx, disk)
		}
	}
	key0 := fmt.Sprintf("/minicloud/db/meta/server/project/%s/name/%s", entity.ProjectId, entity.Name)
	txn.CreateMeta(ctx, key0, entity.Id.String())
	ServerFSM.Notify(ctx, txn, entity)
	return txn.Commit(ctx)
}
func (m *ServerManager) Update(ctx context.Context, entity *db.Server, initiator db.Initiator) error {
	origEntity := entity.Original.(*db.Server)
	if err := ServerFSM.CheckTransition(origEntity.State, entity.State, initiator); err != nil {
		return err
	}
	if entity.ProjectId != origEntity.ProjectId {
		return &db.FieldError{Entity: "server", Field: "ProjectId", Message: "Field change prohibited"}
	}
	if entity.FlavorId != origEntity.FlavorId {
		return &db.FieldError{Entity: "server", Field: "FlavorId", Message: "Field change prohibited"}
	}
	if !utils.ULIDSliceEqual(entity.DiskIds, origEntity.DiskIds) {
		return &db.FieldError{Entity: "server", Field: "DiskIds", Message: "Field change prohibited"}
	}
	if !regexpServerName.MatchString(entity.Name) {
		return &db.FieldError{Entity: "server", Field: "Name", Message: "Should contain only lowercase letters, digits and dash, but shouldn't start or end with dash"}
	}
	txn := m.conn.NewTransaction()
	txn.Update(ctx, entity)
	if entity.ProjectId != origEntity.ProjectId || entity.Name != origEntity.Name {
		forfeitKey0 := fmt.Sprintf("/minicloud/db/meta/server/project/%s/name/%s", origEntity.ProjectId, origEntity.Name)
		txn.CheckMeta(ctx, forfeitKey0, origEntity.Id.String())
		txn.DeleteMeta(ctx, forfeitKey0)
		claimKey0 := fmt.Sprintf("/minicloud/db/meta/server/project/%s/name/%s", entity.ProjectId, entity.Name)
		txn.CreateMeta(ctx, claimKey0, entity.Id.String())
	}
	ServerFSM.Notify(ctx, txn, entity)
	return txn.Commit(ctx)
}
func (m *ServerManager) IntentDelete(ctx context.Context, id ulid.ULID, initiator db.Initiator) error {
	entity, err := Servers(m.conn).Get(ctx, id)
	if err != nil {
		return err
	}
	if err := ServerFSM.CheckTransition(entity.State, db.StateDeleting, initiator); err != nil {
		return err
	}
	entity.State = db.StateDeleting
	txn := m.conn.NewTransaction()
	txn.Update(ctx, entity)
	ServerFSM.Notify(ctx, txn, entity)
	return txn.Commit(ctx)
}
func (m *ServerManager) Delete(ctx context.Context, id ulid.ULID, initiator db.Initiator) error {
	entity, err := Servers(m.conn).Get(ctx, id)
	if err != nil {
		return err
	}
	if err := ServerFSM.CheckTransition(entity.State, db.StateDeleted, initiator); err != nil {
		return err
	}
	txn := m.conn.NewTransaction()
	txn.Delete(ctx, entity)
	if project, err := Projects(m.conn).Get(ctx, entity.ProjectId); err != nil {
		return err
	} else {
		project.ServerIds = utils.RemoveULID(project.ServerIds, entity.Id)
		txn.Update(ctx, project)
	}
	if flavor, err := Flavors(m.conn).Get(ctx, entity.FlavorId); err != nil {
		return err
	} else {
		flavor.ServerIds = utils.RemoveULID(flavor.ServerIds, entity.Id)
		txn.Update(ctx, flavor)
	}
	for _, refEntityId := range entity.DiskIds {
		if disk, err := Disks(m.conn).Get(ctx, refEntityId); err != nil {
			return err
		} else {
			disk.ServerId = utils.Zero
			if err := DiskFSM.ChangeState(disk, db.StateReady, db.InitiatorSystem); err != nil {
				return err
			}
			txn.Update(ctx, disk)
		}
	}
	key0 := fmt.Sprintf("/minicloud/db/meta/server/project/%s/name/%s", entity.ProjectId, entity.Name)
	txn.CheckMeta(ctx, key0, entity.Id.String())
	txn.DeleteMeta(ctx, key0)
	ServerFSM.DeleteNotification(ctx, txn, entity)
	return txn.Commit(ctx)
}
