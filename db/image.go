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
	"github.com/antonf/minicloud/utils"
	"github.com/oklog/ulid"
	"regexp"
)

type ImageManager interface {
	NewEntity() *Image
	Get(ctx context.Context, id ulid.ULID) (*Image, error)
	Create(ctx context.Context, img *Image) error
	Update(ctx context.Context, img *Image) error
	Delete(ctx context.Context, id ulid.ULID) error
}

type Image struct {
	EntityHeader
	Id        ulid.ULID
	ProjectId ulid.ULID
	Name      string
}

var regexpImageName = regexp.MustCompile("[a-zA-Z0-9_.:-]{3,}")

func (img *Image) String() string {
	return fmt.Sprintf(
		"Image{Id:%s Name:%s [%d,%d,%d]}",
		img.Id, img.Name, img.SchemaVersion, img.CreateRev, img.ModifyRev)
}

func (img *Image) validate() error {
	if err := checkFieldRegexp("image", "Name", img.Name, regexpImageName); err != nil {
		return err
	}
	return nil
}

func (img *Image) validateUpdate() error {
	origVal := img.original.(*Image)
	if img.Id != origVal.Id {
		return &FieldError{"image", "Id", "Field is read-only"}
	}
	if img.ProjectId != origVal.ProjectId {
		return &FieldError{"image", "ProjectId", "Field is read-only"}
	}
	return img.validate()
}

func (img *Image) claimUniqueName(txn Transaction) {
	txn.ClaimUnique(img, "name", img.ProjectId.String(), img.Name)
}

func (img *Image) forfeitUniqueName(txn Transaction) {
	txn.ForfeitUnique(img, "name", img.ProjectId.String(), img.Name)
}

type etcdImageManager struct {
	conn *etcdConeection
}

func (pm *etcdImageManager) NewEntity() *Image {
	return &Image{
		EntityHeader: EntityHeader{SchemaVersion: 1},
	}
}

func (im *etcdImageManager) Get(ctx context.Context, id ulid.ULID) (*Image, error) {
	img := &Image{Id: id}
	if err := im.conn.loadEntity(ctx, img); err != nil {
		return nil, err
	}
	return img, nil
}

func (im *etcdImageManager) Create(ctx context.Context, img *Image) error {
	if err := img.validate(); err != nil {
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
	img.claimUniqueName(txn)
	return txn.Commit(ctx)
}

func (im *etcdImageManager) Update(ctx context.Context, img *Image) error {
	if err := img.validateUpdate(); err != nil {
		return err
	}
	c := im.conn
	origImg := img.original.(*Image)
	txn := c.NewTransaction()
	if origImg.Name != img.Name {
		origImg.forfeitUniqueName(txn)
		img.claimUniqueName(txn)
	}
	txn.Update(img)
	return txn.Commit(ctx)
}

func (im *etcdImageManager) Delete(ctx context.Context, id ulid.ULID) error {
	img, err := im.Get(ctx, id)
	if err != nil {
		return nil
	}
	c := im.conn
	proj, err := c.Projects().Get(ctx, img.ProjectId)
	if err != nil {
		return err
	}
	proj.ImageIds = utils.RemoveULID(proj.ImageIds, img.Id)

	txn := c.NewTransaction()
	img.forfeitUniqueName(txn)
	txn.Delete(img)
	txn.Update(proj)
	return txn.Commit(ctx)
}
