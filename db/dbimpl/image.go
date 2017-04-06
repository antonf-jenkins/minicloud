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
	"reflect"
	"regexp"
)

var (
	regexpImageName = regexp.MustCompile("[a-zA-Z0-9_.:-]{3,}")
	imageFSM        = fsm.NewStateMachine().
			InitialState(db.StateCreated).
			UserTransition(db.StateCreated, db.StateCreated). // Allow update in created state
			UserTransition(db.StateReady, db.StateReady).     // Allow update in ready state
			SystemTransition(db.StateCreated, db.StateUploading).
			SystemTransition(db.StateUploading, db.StateReady).
			SystemTransition(db.StateCreated, db.StateError).
			SystemTransition(db.StateUploading, db.StateError).
			SystemTransition(db.StateReady, db.StateError)
)

func validateImage(img *db.Image) error {
	if err := checkFieldRegexp("image", "Name", img.Name, regexpImageName); err != nil {
		return err
	}
	if img.Checksum != "" {
		return &db.FieldError{"image", "Checksum", "Field is read-only"}
	}
	if err := imageFSM.CheckInitialState(img.State); err != nil {
		return err
	}
	if len(img.DiskIds) > 0 {
		return &db.FieldError{"image", "DiskIds", "Should be empty"}
	}
	return nil
}

func validateUpdateImage(img *db.Image, initiator db.Initiator) error {
	origImg := img.Original.(*db.Image)
	if img.Id != origImg.Id {
		return &db.FieldError{"image", "Id", "Field is read-only"}
	}
	if img.ProjectId != origImg.ProjectId {
		return &db.FieldError{"image", "ProjectId", "Field is read-only"}
	}
	if err := imageFSM.CheckTransition(origImg.State, img.State, initiator); err != nil {
		return err
	}
	if err := checkFieldRegexp("image", "Name", img.Name, regexpImageName); err != nil {
		return err
	}
	if initiator != db.InitiatorSystem {
		if !reflect.DeepEqual(origImg.DiskIds, img.DiskIds) {
			return &db.FieldError{"image", "DiskIds", "Field is read-only"}
		}
		if img.Checksum != origImg.Checksum {
			return &db.FieldError{"image", "Checksum", "Field is read-only"}
		}
	}
	return nil
}

func claimUniqueImageName(img *db.Image, txn db.Transaction) {
	txn.ClaimUnique(img, "name", img.ProjectId.String(), img.Name)
}

func forfeitUniqueImageName(img *db.Image, txn db.Transaction) {
	txn.ForfeitUnique(img, "name", img.ProjectId.String(), img.Name)
}

type etcdImageManager struct {
	conn *etcdConeection
}

func (pm *etcdImageManager) NewEntity() *db.Image {
	return &db.Image{
		EntityHeader: db.EntityHeader{
			SchemaVersion: 1,
			State:         db.StateCreated,
		},
	}
}

func (im *etcdImageManager) Get(ctx context.Context, id ulid.ULID) (*db.Image, error) {
	img := &db.Image{EntityHeader: db.EntityHeader{Id: id}}
	if err := im.conn.loadEntity(ctx, img); err != nil {
		return nil, err
	}
	return img, nil
}

func (im *etcdImageManager) Create(ctx context.Context, img *db.Image) error {
	if err := validateImage(img); err != nil {
		return err
	}
	c := im.conn
	proj, err := c.Projects().Get(ctx, img.ProjectId)
	if err != nil {
		return err
	}

	img.Id = utils.NewULID()
	proj.ImageIds = append(proj.ImageIds, img.Id)

	txn := c.NewTransaction()
	txn.Create(img)
	txn.Update(proj)
	claimUniqueImageName(img, txn)
	return txn.Commit(ctx)
}

func (im *etcdImageManager) Update(ctx context.Context, img *db.Image, initiator db.Initiator) error {
	if err := validateUpdateImage(img, initiator); err != nil {
		return err
	}
	c := im.conn
	origImg := img.Original.(*db.Image)
	txn := c.NewTransaction()
	if origImg.Name != img.Name {
		forfeitUniqueImageName(origImg, txn)
		claimUniqueImageName(img, txn)
	}
	txn.Update(img)
	return txn.Commit(ctx)
}

func (im *etcdImageManager) Delete(ctx context.Context, id ulid.ULID) error {
	img, err := im.Get(ctx, id)
	if err != nil {
		return nil
	}
	if len(img.DiskIds) != 0 {
		return &db.FieldError{"image", "DiskIds", "Can't delete image referenced by disk"}
	}

	c := im.conn
	proj, err := c.Projects().Get(ctx, img.ProjectId)
	if err != nil {
		return err
	}
	proj.ImageIds = utils.RemoveULID(proj.ImageIds, img.Id)

	txn := c.NewTransaction()
	forfeitUniqueImageName(img, txn)
	txn.Delete(img)
	txn.Update(proj)
	return txn.Commit(ctx)
}

func (im *etcdImageManager) Watch(ctx context.Context) chan *db.Image {
	entityCh := im.conn.watchEntity(ctx, func() db.Entity { return im.NewEntity() })
	resultCh := make(chan *db.Image)
	go func() {
	loop:
		for {
			select {
			case entity := <-entityCh:
				if entity == nil {
					break loop
				}
				resultCh <- entity.(*db.Image)
			case <-ctx.Done():
				break loop
			}
		}
		close(resultCh)
	}()
	return resultCh
}
