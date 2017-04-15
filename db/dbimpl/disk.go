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
)

func validateDisk(disk *db.Disk) error {
	if err := fsm.DiskFSM.CheckInitialState(disk.State); err != nil {
		return err
	}
	return nil
}

func validateUpdateDisk(disk *db.Disk, initiator db.Initiator) error {
	origDisk := disk.Original.(*db.Disk)
	if disk.Id != origDisk.Id {
		return &db.FieldError{"disk", "Id", "Field is read-only"}
	}
	if disk.ProjectId != origDisk.ProjectId {
		return &db.FieldError{"disk", "ProjectId", "Field is read-only"}
	}
	if disk.ImageId != origDisk.ImageId {
		return &db.FieldError{"disk", "ImageId", "Field is read-only"}
	}
	if err := fsm.DiskFSM.CheckTransition(origDisk.State, disk.State, initiator); err != nil {
		return err
	}
	return nil
}

type etcdDiskManager struct {
	conn *etcdConnection
}

func (dm *etcdDiskManager) NewEntity() *db.Disk {
	return &db.Disk{
		EntityHeader: db.EntityHeader{
			SchemaVersion: 1,
			State:         db.StateCreated,
		},
	}
}

func (dm *etcdDiskManager) Get(ctx context.Context, id ulid.ULID) (*db.Disk, error) {
	disk := &db.Disk{EntityHeader: db.EntityHeader{Id: id}}
	if err := dm.conn.loadEntity(ctx, disk); err != nil {
		return nil, err
	}
	return disk, nil
}

func (dm *etcdDiskManager) Create(ctx context.Context, disk *db.Disk) error {
	if err := validateDisk(disk); err != nil {
		return err
	}

	c := dm.conn
	proj, err := c.Projects().Get(ctx, disk.ProjectId)
	if err != nil {
		return err
	}
	var img *db.Image
	if disk.ImageId != utils.Zero {
		if img, err = c.Images().Get(ctx, disk.ImageId); err != nil {
			return err
		}
		if img.State != db.StateReady {
			logger.Error(ctx, "image in invalid state", "image_id", disk.ImageId, "state", img.State)
			return &fsm.InvalidStateError{State: img.State}
		}
	}

	disk.Id = utils.NewULID()
	proj.DiskIds = append(proj.DiskIds, disk.Id)
	if img != nil {
		img.DiskIds = append(img.DiskIds, disk.Id)
	}

	txn := c.NewTransaction()
	txn.Create(ctx, disk)
	txn.Update(ctx, proj)
	if img != nil {
		txn.Update(ctx, img)
	}
	fsm.DiskFSM.Notify(ctx, txn, disk)
	return txn.Commit(ctx)
}

func (dm *etcdDiskManager) Update(ctx context.Context, disk *db.Disk, initiator db.Initiator) error {
	if err := validateUpdateDisk(disk, initiator); err != nil {
		return err
	}
	c := dm.conn
	txn := c.NewTransaction()
	txn.Update(ctx, disk)
	fsm.DiskFSM.Notify(ctx, txn, disk)
	return txn.Commit(ctx)
}

func (dm *etcdDiskManager) IntentDelete(ctx context.Context, id ulid.ULID, initiator db.Initiator) error {
	disk, err := dm.Get(ctx, id)
	if err != nil {
		return err
	}

	if err := fsm.DiskFSM.CheckTransition(disk.State, db.StateDeleting, initiator); err != nil {
		return err
	}
	disk.State = db.StateDeleting

	txn := dm.conn.NewTransaction()
	txn.Update(ctx, disk)
	fsm.DiskFSM.Notify(ctx, txn, disk)
	return txn.Commit(ctx)
}

func (dm *etcdDiskManager) Delete(ctx context.Context, id ulid.ULID, initiator db.Initiator) error {
	disk, err := dm.Get(ctx, id)
	if err != nil {
		return err
	}
	if err := fsm.ImageFSM.CheckTransition(disk.State, db.StateDeleted, initiator); err != nil {
		return err
	}

	c := dm.conn
	proj, err := c.Projects().Get(ctx, disk.ProjectId)
	if err != nil {
		return err
	}
	proj.DiskIds = utils.RemoveULID(proj.DiskIds, disk.Id)

	var img *db.Image
	if disk.ImageId != utils.Zero {
		if img, err = c.Images().Get(ctx, disk.ImageId); err != nil {
			return err
		}
		img.DiskIds = utils.RemoveULID(img.DiskIds, disk.Id)
	}

	txn := c.NewTransaction()
	txn.Delete(ctx, disk)
	txn.Update(ctx, proj)
	if img != nil {
		txn.Update(ctx, img)
	}
	fsm.DiskFSM.DeleteNotification(ctx, txn, disk)
	return txn.Commit(ctx)
}
