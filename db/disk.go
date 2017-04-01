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
	"fmt"
	"github.com/antonf/minicloud/fsm"
	"github.com/antonf/minicloud/utils"
	"github.com/oklog/ulid"
)

type DiskManager interface {
	NewEntity() *Disk
	Get(ctx context.Context, id ulid.ULID) (*Disk, error)
	Create(ctx context.Context, disk *Disk) error
	Update(ctx context.Context, disk *Disk, initiator fsm.Initiator) error
	Delete(ctx context.Context, id ulid.ULID) error
	Watch(ctx context.Context) chan *Disk
}

type Disk struct {
	EntityHeader
	Id        ulid.ULID
	ProjectId ulid.ULID
	Desc      string
	State     fsm.State
	Pool      string
	ImageId   ulid.ULID
	Size      int64
}

var (
	diskFSM = fsm.NewStateMachine().
		InitialState(fsm.StateCreated).
		UserTransition(fsm.StateReady, fsm.StateUpdated).
		SystemTransition(fsm.StateCreated, fsm.StateReady).
		SystemTransition(fsm.StateUpdated, fsm.StateReady).
		SystemTransition(fsm.StateInUse, fsm.StateReady).
		SystemTransition(fsm.StateReady, fsm.StateInUse).
		SystemTransition(fsm.StateCreated, fsm.StateError).
		SystemTransition(fsm.StateReady, fsm.StateError).
		SystemTransition(fsm.StateUpdated, fsm.StateError).
		SystemTransition(fsm.StateInUse, fsm.StateError)
)

func (disk *Disk) String() string {
	return fmt.Sprintf(
		"Disk{Id:%s [%d,%d,%d]}",
		disk.Id, disk.SchemaVersion, disk.CreateRev, disk.ModifyRev)
}

func (disk *Disk) validate() error {
	if err := diskFSM.CheckInitialState(disk.State); err != nil {
		return err
	}
	return nil
}

func (disk *Disk) validateUpdate(initiator fsm.Initiator) error {
	origDisk := disk.original.(*Disk)
	if disk.Id != origDisk.Id {
		return &FieldError{"disk", "Id", "Field is read-only"}
	}
	if disk.ProjectId != origDisk.ProjectId {
		return &FieldError{"disk", "ProjectId", "Field is read-only"}
	}
	if disk.ImageId != origDisk.ImageId {
		return &FieldError{"disk", "ImageId", "Field is read-only"}
	}
	if err := diskFSM.CheckTransition(origDisk.State, disk.State, initiator); err != nil {
		return err
	}
	return nil
}

type etcdDiskManager struct {
	conn *etcdConeection
}

func (dm *etcdDiskManager) NewEntity() *Disk {
	return &Disk{
		EntityHeader: EntityHeader{SchemaVersion: 1},
		State:        fsm.StateCreated,
	}
}

func (dm *etcdDiskManager) Get(ctx context.Context, id ulid.ULID) (*Disk, error) {
	disk := &Disk{Id: id}
	if err := dm.conn.loadEntity(ctx, disk); err != nil {
		return nil, err
	}
	return disk, nil
}

func (dm *etcdDiskManager) Create(ctx context.Context, disk *Disk) error {
	if err := disk.validate(); err != nil {
		return err
	}

	c := dm.conn
	proj, err := c.Projects().Get(ctx, disk.ProjectId)
	if err != nil {
		return err
	}
	var img *Image
	if disk.ImageId != utils.Zero {
		if img, err = c.Images().Get(ctx, disk.ImageId); err != nil {
			return err
		}
	}

	disk.Id = utils.NewULID()
	proj.DiskIds = append(proj.DiskIds, disk.Id)
	if img != nil {
		img.DiskIds = append(img.DiskIds, disk.Id)
	}

	txn := c.NewTransaction()
	txn.Create(disk)
	txn.Update(proj)
	if img != nil {
		txn.Update(img)
	}
	return txn.Commit(ctx)
}

func (dm *etcdDiskManager) Update(ctx context.Context, disk *Disk, initiator fsm.Initiator) error {
	if err := disk.validateUpdate(initiator); err != nil {
		return err
	}
	c := dm.conn
	txn := c.NewTransaction()
	txn.Update(disk)
	return txn.Commit(ctx)
}

func (dm *etcdDiskManager) Delete(ctx context.Context, id ulid.ULID) error {
	disk, err := dm.Get(ctx, id)
	if err != nil {
		return nil
	}
	c := dm.conn
	proj, err := c.Projects().Get(ctx, disk.ProjectId)
	if err != nil {
		return err
	}
	proj.DiskIds = utils.RemoveULID(proj.DiskIds, disk.Id)

	txn := c.NewTransaction()
	txn.Delete(disk)
	txn.Update(proj)
	return txn.Commit(ctx)
}

func (dm *etcdDiskManager) Watch(ctx context.Context) chan *Disk {
	entityCh := dm.conn.watchEntity(ctx, func() Entity { return dm.NewEntity() })
	resultCh := make(chan *Disk)
	go func() {
	loop:
		for {
			select {
			case entity := <-entityCh:
				if entity == nil {
					break loop
				}
				resultCh <- entity.(*Disk)
			case <-ctx.Done():
				break loop
			}
		}
		close(resultCh)
	}()
	return resultCh
}
