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
	"reflect"
)

var regexpProjectName = regexp.MustCompile("[a-zA-Z0-9_.:-]{3,}")

type Project struct {
	EntityHeader
	Id   ulid.ULID
	Name string
	ImageIds []ulid.ULID
}

func (p Project) String() string {
	return fmt.Sprintf(
		"Project{Id:%s Name:%s [%d,%d,%d]}",
		p.Id, p.Name, p.SchemaVersion, p.CreateRev, p.ModifyRev)
}

func (p *Project) validate() error {
	if err := checkFieldRegexp("project", "Name", p.Name, regexpProjectName); err != nil {
		return err
	}
	if len(p.ImageIds) > 0 {
		return &FieldError{"project", "ImageIds", "Should be empty"}
	}
	return nil
}

func (p *Project) validateUpdate() error {
	if err := checkFieldRegexp("project", "Name", p.Name, regexpProjectName); err != nil {
		return err
	}
	origVal := p.original.(*Project)
	if p.Id != origVal.Id {
		return &FieldError{"project", "Id", "Field is read-only"}
	}
	if !reflect.DeepEqual(origVal.ImageIds, p.ImageIds) {
		return &FieldError{"project", "ImageIds", "Field is read-only"}
	}
	return nil
}

func (p *Project) claimUniqueName(txn Transaction) {
	txn.ClaimUnique(p, "name", p.Name)
}

func (p *Project) forfeitUniqueName(txn Transaction) {
	txn.ForfeitUnique(p, "name", p.Name)
}

func (c *etcdConeection) GetProject(ctx context.Context, id ulid.ULID) (*Project, error) {
	proj := &Project{Id: id}
	if err := c.loadEntity(ctx, proj); err != nil {
		return nil, err
	}
	return proj, nil
}

func (c *etcdConeection) CreateProject(ctx context.Context, proj *Project) error {
	if err := proj.validate(); err != nil {
		return err
	}
	proj.Id = utils.NewULID()
	txn := c.NewTransaction()
	txn.Create(proj)
	proj.claimUniqueName(txn)
	return txn.Commit(ctx)
}

func (c *etcdConeection) UpdateProject(ctx context.Context, proj *Project) error {
	if err := proj.validateUpdate(); err != nil {
		return err
	}
	origProj := proj.original.(*Project)
	txn := c.NewTransaction()
	txn.Update(proj)
	if origProj.Name != proj.Name {
		origProj.forfeitUniqueName(txn)
		proj.claimUniqueName(txn)
	}
	return txn.Commit(ctx)
}

func (c *etcdConeection) DeleteProject(ctx context.Context, id ulid.ULID) error {
	proj, err := c.GetProject(ctx, id)
	if err != nil {
		return nil
	}
	if len(proj.ImageIds) != 0 {
		return &FieldError{"project", "ImageIds", "Can't delete non-empty project"}
	}
	txn := c.NewTransaction()
	proj.forfeitUniqueName(txn)
	txn.Delete(proj)
	return txn.Commit(ctx)
}
