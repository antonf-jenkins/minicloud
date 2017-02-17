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

var regexpProjectName = regexp.MustCompile("[a-zA-Z0-9_.:-]{3,}")

type Project struct {
	EntityHeader
	Id   ulid.ULID
	Name string
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
	return nil
}

func (p *Project) validateUpdate() error {
	origVal := p.original.(*Project)
	if p.Id != origVal.Id {
		return &FieldError{"project", "Id", "Field is read-only"}
	}
	return p.validate()
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
	txn.ClaimUnique(proj, "Name")
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
		txn.ForfeitUnique(origProj, "Name")
		txn.ClaimUnique(proj, "Name")
	}
	return txn.Commit(ctx)
}

func (c *etcdConeection) DeleteProject(ctx context.Context, id ulid.ULID) error {
	proj, err := c.GetProject(ctx, id)
	if err != nil {
		return nil
	}
	txn := c.NewTransaction()
	txn.ForfeitUnique(proj, "Name")
	txn.Delete(proj)
	return txn.Commit(ctx)
}
