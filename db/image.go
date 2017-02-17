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
	"github.com/oklog/ulid"
	"regexp"
	"fmt"
	"context"
	"github.com/antonf/minicloud/utils"
)

type Image struct {
	EntityHeader
	Id ulid.ULID
	ProjectId ulid.ULID
	Name string
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

func (c *etcdConeection) GetImage(ctx context.Context, id ulid.ULID) (*Image, error) {
	img := &Image{Id: id}
	if err := c.loadEntity(ctx, img); err != nil {
		return nil, err
	}
	return img, nil
}

func (c *etcdConeection) CreateImage(ctx context.Context, img *Image) error {
	if err := img.validate(); err != nil {
		return err
	}
	proj, err := c.GetProject(ctx, img.ProjectId)
	if err != nil {
		return err
	}

	img.Id = utils.NewULID()
	proj.ImageIds = append(proj.ImageIds, img.Id)

	txn := c.NewTransaction()
	txn.Create(img)
	txn.Update(proj)
	return txn.Commit(ctx)
}

func (c *etcdConeection) UpdateImage(ctx context.Context, img *Image) error {
	if err := img.validateUpdate(); err != nil {
		return err
	}
	txn := c.NewTransaction()
	txn.Update(img)
	return txn.Commit(ctx)
}

func (c *etcdConeection) DeleteImage(ctx context.Context, id ulid.ULID) error {
	img, err := c.GetImage(ctx, id)
	if err != nil {
		return nil
	}
	proj, err := c.GetProject(ctx, img.ProjectId)
	if err != nil {
		return err
	}
	proj.ImageIds = utils.RemoveULID(proj.ImageIds, img.Id)

	txn := c.NewTransaction()
	txn.Delete(img)
	txn.Update(proj)
	return txn.Commit(ctx)
}
