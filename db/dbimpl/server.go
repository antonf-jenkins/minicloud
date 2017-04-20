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
	"github.com/antonf/minicloud/db"
	"github.com/antonf/minicloud/fsm"
	"github.com/antonf/minicloud/utils"
	"github.com/oklog/ulid"
	"regexp"
)

var regexpServerName = regexp.MustCompile("^[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?$")
var regexpServerNameAllNum = regexp.MustCompile("^[0-9]+$")

func validateServerFields(server *db.Server) error {
	if err := checkFieldRegexp("server", "Name", server.Name, regexpServerName); err != nil {
		return err
	}
	if regexpServerNameAllNum.MatchString(server.Name) {
		return &db.FieldError{Entity: "server", Field: "Name", Message: "name should have at least one letter"}
	}
	return nil
}

func claimUniqueServerName(ctx context.Context, server *db.Server, txn db.Transaction) {
	// TODO: check for uniqueness in domain name. Name checked globally for now.
	txn.CreateMeta(ctx, uniqueMetaKey(server, "name", server.Name), server.Id.String())
}

func forfeitUniqueServerName(ctx context.Context, server *db.Server, txn db.Transaction) {
	// TODO: check for uniqueness in domain name. Name checked globally for now.
	key := uniqueMetaKey(server, "name", server.Name)
	txn.CheckMeta(ctx, key, server.Id.String())
	txn.DeleteMeta(ctx, key)
}

type etcdServerManager struct {
	conn *etcdConnection
}

func (sm *etcdServerManager) NewEntity() *db.Server {
	return &db.Server{
		EntityHeader: db.EntityHeader{
			SchemaVersion: 1,
			State:         db.StateCreated,
		},
	}
}

func (sm *etcdServerManager) Get(ctx context.Context, id ulid.ULID) (*db.Server, error) {
	server := &db.Server{EntityHeader: db.EntityHeader{Id: id}}
	if err := sm.conn.loadEntity(ctx, server); err != nil {
		return nil, err
	}
	return server, nil
}

func (sm *etcdServerManager) Create(ctx context.Context, server *db.Server) error {
	if err := validateServerFields(server); err != nil {
		return err
	}

	c := sm.conn
	proj, err := c.Projects().Get(ctx, server.ProjectId)
	if err != nil {
		return err
	}
	flavor, err := c.Flavors().Get(ctx, server.FlavorId)
	if err != nil {
		return err
	}
	disks := make([]*db.Disk, len(server.DiskIds))
	for i, diskId := range server.DiskIds {
		if disks[i], err = c.Disks().Get(ctx, diskId); err != nil {
			return err
		}
	}

	server.Id = utils.NewULID()
	proj.ServerIds = append(proj.ServerIds, server.Id)
	flavor.ServerIds = append(flavor.ServerIds, server.Id)
	for _, disk := range disks {
		if err := fsm.DiskFSM.ChangeState(disk, db.StateInUse, db.InitiatorSystem); err != nil {
			return err
		}
		disk.ServerId = server.Id
	}

	txn := c.NewTransaction()
	txn.Create(ctx, server)
	txn.Update(ctx, proj)
	txn.Update(ctx, flavor)
	for _, disk := range disks {
		txn.Update(ctx, disk)
	}
	claimUniqueServerName(ctx, server, txn)
	fsm.ServerFSM.Notify(ctx, txn, server)
	return txn.Commit(ctx)
}

func (sm *etcdServerManager) Update(ctx context.Context, server *db.Server, initiator db.Initiator) error {
	if err := validateServerFields(server); err != nil {
		return err
	}
	if err := checkReadOnlyFields(server, "ProjectId", "FlavorId", "DiskIds"); err != nil {
		return err
	}

	origServer := server.Original.(*db.Server)
	if err := fsm.ServerFSM.CheckTransition(origServer.State, server.State, initiator); err != nil {
		return err
	}

	c := sm.conn
	txn := c.NewTransaction()
	if origServer.Name != server.Name {
		forfeitUniqueServerName(ctx, origServer, txn)
		claimUniqueServerName(ctx, server, txn)
	}
	txn.Update(ctx, server)
	fsm.ServerFSM.Notify(ctx, txn, server)
	return txn.Commit(ctx)
}

func (sm *etcdServerManager) IntentDelete(ctx context.Context, id ulid.ULID, initiator db.Initiator) error {
	server, err := sm.Get(ctx, id)
	if err != nil {
		return err
	}

	if err := fsm.ServerFSM.ChangeState(server, db.StateDeleting, initiator); err != nil {
		return err
	}

	txn := sm.conn.NewTransaction()
	txn.Update(ctx, server)
	fsm.ServerFSM.Notify(ctx, txn, server)
	return txn.Commit(ctx)
}

func (sm *etcdServerManager) Delete(ctx context.Context, id ulid.ULID, initiator db.Initiator) error {
	server, err := sm.Get(ctx, id)
	if err != nil {
		return err
	}
	if err := fsm.ImageFSM.CheckTransition(server.State, db.StateDeleted, initiator); err != nil {
		return err
	}

	c := sm.conn
	proj, err := c.Projects().Get(ctx, server.ProjectId)
	if err != nil {
		return err
	}
	flavor, err := c.Flavors().Get(ctx, server.FlavorId)
	if err != nil {
		return err
	}
	disks := make([]*db.Disk, len(server.DiskIds))
	for i, diskId := range server.DiskIds {
		if disks[i], err = c.Disks().Get(ctx, diskId); err != nil {
			return err
		}
	}

	// Update back references
	proj.ServerIds = utils.RemoveULID(proj.ServerIds, server.Id)
	flavor.ServerIds = utils.RemoveULID(flavor.ServerIds, server.Id)
	for _, disk := range disks {
		if err := fsm.DiskFSM.ChangeState(disk, db.StateReady, initiator); err != nil {
			return err
		}
		disk.ServerId = utils.Zero
	}

	txn := c.NewTransaction()
	txn.Delete(ctx, server)
	txn.Update(ctx, proj)
	txn.Update(ctx, flavor)
	for _, disk := range disks {
		txn.Update(ctx, disk)
	}
	forfeitUniqueServerName(ctx, server, txn)
	fsm.ServerFSM.DeleteNotification(ctx, txn, server)
	return txn.Commit(ctx)
}
