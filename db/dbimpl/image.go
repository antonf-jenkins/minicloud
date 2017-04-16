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
	regexpImageName = regexp.MustCompile("^[a-zA-Z0-9_.:-]{3,}$")
)

func validateImage(img *db.Image) error {
	if err := checkFieldRegexp("image", "Name", img.Name, regexpImageName); err != nil {
		return err
	}
	if img.Checksum != "" {
		return &db.FieldError{Entity: "image", Field: "Checksum", Message: "Field is read-only"}
	}
	if err := fsm.ImageFSM.CheckInitialState(img.State); err != nil {
		return err
	}
	if len(img.DiskIds) > 0 {
		return &db.FieldError{Entity: "image", Field: "DiskIds", Message: "Should be empty"}
	}
	return nil
}

func validateUpdateImage(img *db.Image, initiator db.Initiator) error {
	origImg := img.Original.(*db.Image)
	if img.Id != origImg.Id {
		return &db.FieldError{Entity: "image", Field: "Id", Message: "Field is read-only"}
	}
	if img.ProjectId != origImg.ProjectId {
		return &db.FieldError{Entity: "image", Field: "ProjectId", Message: "Field is read-only"}
	}
	if err := fsm.ImageFSM.CheckTransition(origImg.State, img.State, initiator); err != nil {
		return err
	}
	if err := checkFieldRegexp("image", "Name", img.Name, regexpImageName); err != nil {
		return err
	}
	if initiator != db.InitiatorSystem {
		if !reflect.DeepEqual(origImg.DiskIds, img.DiskIds) {
			return &db.FieldError{Entity: "image", Field: "DiskIds", Message: "Field is read-only"}
		}
		if img.Checksum != origImg.Checksum {
			return &db.FieldError{Entity: "image", Field: "Checksum", Message: "Field is read-only"}
		}
	}
	return nil
}

func claimUniqueImageName(ctx context.Context, img *db.Image, txn db.Transaction) {
	txn.CreateMeta(ctx, uniqueMetaKey(img, "name", img.ProjectId.String(), img.Name), img.Id.String())
}

func forfeitUniqueImageName(ctx context.Context, img *db.Image, txn db.Transaction) {
	key := uniqueMetaKey(img, "name", img.ProjectId.String(), img.Name)
	txn.CheckMeta(ctx, key, img.Id.String())
	txn.DeleteMeta(ctx, key)
}

type etcdImageManager struct {
	conn *etcdConnection
}

func (im *etcdImageManager) NewEntity() *db.Image {
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
	txn.Create(ctx, img)
	txn.Update(ctx, proj)
	claimUniqueImageName(ctx, img, txn)
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
		forfeitUniqueImageName(ctx, origImg, txn)
		claimUniqueImageName(ctx, img, txn)
	}
	txn.Update(ctx, img)
	return txn.Commit(ctx)
}

func (im *etcdImageManager) IntentDelete(ctx context.Context, id ulid.ULID, initiator db.Initiator) error {
	img, err := im.Get(ctx, id)
	if err != nil {
		return err
	}
	if len(img.DiskIds) != 0 {
		return &db.FieldError{Entity: "image", Field: "DiskIds", Message: "Can't delete image referenced by disk"}
	}

	if err := fsm.ImageFSM.CheckTransition(img.State, db.StateDeleting, initiator); err != nil {
		return err
	}
	img.State = db.StateDeleting

	txn := im.conn.NewTransaction()
	forfeitUniqueImageName(ctx, img, txn)
	txn.Update(ctx, img)
	return txn.Commit(ctx)
}

func (im *etcdImageManager) Delete(ctx context.Context, id ulid.ULID, initiator db.Initiator) error {
	img, err := im.Get(ctx, id)
	if err != nil {
		return err
	}
	if len(img.DiskIds) != 0 {
		return &db.FieldError{Entity: "image", Field: "DiskIds", Message: "Can't delete image referenced by disk"}
	}
	if err := fsm.ImageFSM.CheckTransition(img.State, db.StateDeleted, initiator); err != nil {
		return err
	}

	c := im.conn
	proj, err := c.Projects().Get(ctx, img.ProjectId)
	if err != nil {
		return err
	}
	proj.ImageIds = utils.RemoveULID(proj.ImageIds, img.Id)

	txn := c.NewTransaction()
	forfeitUniqueImageName(ctx, img, txn)
	txn.Delete(ctx, img)
	txn.Update(ctx, proj)
	return txn.Commit(ctx)
}
