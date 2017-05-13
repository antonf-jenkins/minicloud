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
	"github.com/antonf/minicloud/ceph"
	"github.com/antonf/minicloud/db"
	"github.com/antonf/minicloud/utils"
)

var DiskFSM *StateMachine

func init() {
	DiskFSM = NewStateMachine().
		InitialState(db.StateCreated).
		UserTransition(db.StateReady, db.StateUpdated).
		UserTransition(db.StateReady, db.StateDeleting).
		UserTransition(db.StateError, db.StateDeleting).
		SystemTransition(db.StateCreated, db.StateReady).
		SystemTransition(db.StateUpdated, db.StateReady).
		SystemTransition(db.StateInUse, db.StateReady).
		SystemTransition(db.StateReady, db.StateInUse).
		SystemTransition(db.StateCreated, db.StateError).
		SystemTransition(db.StateReady, db.StateError).
		SystemTransition(db.StateUpdated, db.StateError).
		SystemTransition(db.StateInUse, db.StateError).
		Hook(db.StateCreated, HandleDiskCreated).
		Hook(db.StateUpdated, HandleDiskUpdated).
		Hook(db.StateDeleting, HandleDiskDeleting)
}

func HandleDiskCreated(ctx context.Context, conn db.Connection, entity db.Entity) {
	disk := entity.(*Disk)
	var err error
	if disk.ImageId != utils.Zero {
		err = ceph.CreateDiskFromImage(ctx, disk.Pool, disk.Id.String(), "images", disk.ImageId.String(), "base", disk.Size)
	} else {
		err = ceph.CreateEmptyDisk(ctx, disk.Pool, disk.Id.String(), disk.Size)
	}
	if err != nil {
		logger.Debug(ctx, "setting disk state to error", "id", disk.Id, "cause", err)
		disk.State = db.StateError
	} else {
		disk.State = db.StateReady
	}
	if err = Disks(conn).Update(ctx, disk, db.InitiatorSystem); err != nil {
		logger.Error(ctx, "failed to change disk state state", "id", disk.Id, "state", disk.State, "error", err)
	}
}

func HandleDiskUpdated(ctx context.Context, conn db.Connection, entity db.Entity) {
	disk := entity.(*Disk)
	if err := ceph.ResizeDisk(ctx, disk.Pool, disk.Id.String(), disk.Size); err != nil {
		logger.Debug(ctx, "setting disk state to error", "id", disk.Id, "cause", err)
		disk.State = db.StateError
	} else {
		disk.State = db.StateReady
	}
	if err := Disks(conn).Update(ctx, disk, db.InitiatorSystem); err != nil {
		logger.Error(ctx, "failed to change disk state state", "id", disk.Id, "state", disk.State, "error", err)
	}
}

func HandleDiskDeleting(ctx context.Context, conn db.Connection, entity db.Entity) {
	disk := entity.(*Disk)
	diskManager := Disks(conn)
	if err := ceph.DeleteDisk(ctx, disk.Pool, disk.Id.String()); err != nil {
		logger.Debug(ctx, "setting disk state to error", "id", disk.Id, "cause", err)
		utils.Retry(ctx, func(ctx context.Context) error {
			disk, err := diskManager.Get(ctx, disk.Id)
			if err != nil {
				return err
			}
			disk.State = db.StateError
			if err := diskManager.Update(ctx, disk, db.InitiatorSystem); err != nil {
				logger.Error(ctx, "failed to change disk state", "id", disk.Id, "state", disk.State, "error", err)
				return err
			}
			return nil
		})
		return
	}
	err := utils.Retry(ctx, func(ctx context.Context) error {
		return diskManager.Delete(ctx, disk.Id, db.InitiatorSystem)
	})
	if err != nil {
		logger.Error(ctx, "failed to delete disk from database", "id", disk.Id, "error", err)
	}
}
