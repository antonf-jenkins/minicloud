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
package fsm

import (
	"context"
	"github.com/antonf/minicloud/ceph"
	"github.com/antonf/minicloud/db"
	"github.com/antonf/minicloud/utils"
	"log"
)

var DiskFSM = NewStateMachine().
	InitialState(db.StateCreated).
	UserTransition(db.StateReady, db.StateUpdated).
	SystemTransition(db.StateCreated, db.StateReady).
	SystemTransition(db.StateUpdated, db.StateReady).
	SystemTransition(db.StateInUse, db.StateReady).
	SystemTransition(db.StateReady, db.StateInUse).
	SystemTransition(db.StateCreated, db.StateError).
	SystemTransition(db.StateReady, db.StateError).
	SystemTransition(db.StateUpdated, db.StateError).
	SystemTransition(db.StateInUse, db.StateError).
	Hook(db.StateCreated, HandleDiskCreated)

func HandleDiskCreated(ctx context.Context, conn db.Connection, entity db.Entity) {
	disk := entity.(*db.Disk)
	var err error
	if disk.ImageId != utils.Zero {
		err = ceph.CreateDiskFromImage(disk.Pool, disk.Id.String(), "images", disk.ImageId.String(), "base", disk.Size)
	} else {
		err = ceph.CreateEmptyDisk(disk.Pool, disk.Id.String(), disk.Size)
	}
	if err != nil {
		disk.State = db.StateError
		log.Printf("fsm: failed to create disk id=%s: %s", disk.Id, err)
	} else {
		disk.State = db.StateReady
	}
	if err = conn.Disks().Update(ctx, disk, db.InitiatorSystem); err != nil {
		log.Printf("fsm: failed to change disk state state=%s id=%s: %s", disk.State, disk.Id, err)
	}
}
