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
)

var ImageFSM = NewStateMachine().
	InitialState(db.StateCreated).
	UserTransition(db.StateCreated, db.StateCreated). // Allow update in created state
	UserTransition(db.StateReady, db.StateReady).     // Allow update in ready state
	UserTransition(db.StateReady, db.StateDeleting).
	UserTransition(db.StateCreated, db.StateDeleting).
	UserTransition(db.StateError, db.StateDeleting).
	SystemTransition(db.StateCreated, db.StateUploading).
	SystemTransition(db.StateUploading, db.StateReady).
	SystemTransition(db.StateCreated, db.StateError).
	SystemTransition(db.StateUploading, db.StateError).
	SystemTransition(db.StateDeleting, db.StateDeleted).
	Hook(db.StateDeleting, HandleImageDeleting)

func HandleImageDeleting(ctx context.Context, conn db.Connection, entity db.Entity) {
	img := entity.(*db.Image)
	if err := ceph.DeleteImage(ctx, "images", img.Id.String()); err != nil {
		logger.Debug(ctx, "setting image state to error", "id", img.Id, "cause", err)
		utils.Retry(ctx, func(ctx context.Context) error {
			img, err := conn.Images().Get(ctx, img.Id)
			if err != nil {
				return err
			}
			img.State = db.StateError
			if err := conn.Images().Update(ctx, img, db.InitiatorSystem); err != nil {
				logger.Error(ctx, "failed to change image state", "id", img.Id, "state", img.State, "error", err)
				return err
			}
			return nil
		})
		return
	}
	err := utils.Retry(ctx, func(ctx context.Context) error {
		return conn.Images().Delete(ctx, img.Id, db.InitiatorSystem)
	})
	if err != nil {
		logger.Error(ctx, "failed to delete image from database", "id", img.Id, "error", err)
	}
}
