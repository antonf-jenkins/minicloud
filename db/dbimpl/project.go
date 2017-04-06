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
	"github.com/antonf/minicloud/utils"
	"github.com/oklog/ulid"
	"reflect"
	"regexp"
)

var regexpProjectName = regexp.MustCompile("[a-zA-Z0-9_.:-]{3,}")

func validateProject(p *db.Project) error {
	if err := checkFieldRegexp("project", "Name", p.Name, regexpProjectName); err != nil {
		return err
	}
	if len(p.ImageIds) > 0 {
		return &db.FieldError{"project", "ImageIds", "Should be empty"}
	}
	if len(p.DiskIds) > 0 {
		return &db.FieldError{"project", "DiskIds", "Should be empty"}
	}
	return nil
}

func validateUpdateProject(p *db.Project, initiator db.Initiator) error {
	if err := checkFieldRegexp("project", "Name", p.Name, regexpProjectName); err != nil {
		return err
	}
	origVal := p.Original.(*db.Project)
	if p.Id != origVal.Id {
		return &db.FieldError{"project", "Id", "Field is read-only"}
	}
	if initiator != db.InitiatorSystem {
		if !reflect.DeepEqual(origVal.ImageIds, p.ImageIds) {
			return &db.FieldError{"project", "ImageIds", "Field is read-only"}
		}
		if !reflect.DeepEqual(origVal.DiskIds, p.DiskIds) {
			return &db.FieldError{"project", "DiskIds", "Field is read-only"}
		}
	}
	return nil
}

func claimUniqueProjectName(p *db.Project, txn db.Transaction) {
	txn.ClaimUnique(p, "name", p.Name)
}

func forfeitUniqueProjectName(p *db.Project, txn db.Transaction) {
	txn.ForfeitUnique(p, "name", p.Name)
}

type etcdProjectManager struct {
	conn *etcdConeection
}

func (pm *etcdProjectManager) NewEntity() *db.Project {
	return &db.Project{
		EntityHeader: db.EntityHeader{SchemaVersion: 1},
	}
}

func (pm *etcdProjectManager) Get(ctx context.Context, id ulid.ULID) (*db.Project, error) {
	proj := &db.Project{EntityHeader: db.EntityHeader{Id: id}}
	if err := pm.conn.loadEntity(ctx, proj); err != nil {
		return nil, err
	}
	return proj, nil
}

func (pm *etcdProjectManager) Create(ctx context.Context, proj *db.Project) error {
	if err := validateProject(proj); err != nil {
		return err
	}
	c := pm.conn
	proj.Id = utils.NewULID()
	txn := c.NewTransaction()
	txn.Create(proj)
	claimUniqueProjectName(proj, txn)
	return txn.Commit(ctx)
}

func (pm *etcdProjectManager) Update(ctx context.Context, proj *db.Project, initiator db.Initiator) error {
	if err := validateUpdateProject(proj, initiator); err != nil {
		return err
	}
	c := pm.conn
	origProj := proj.Original.(*db.Project)
	txn := c.NewTransaction()
	txn.Update(proj)
	if origProj.Name != proj.Name {
		forfeitUniqueProjectName(origProj, txn)
		claimUniqueProjectName(proj, txn)
	}
	return txn.Commit(ctx)
}

func (pm *etcdProjectManager) Delete(ctx context.Context, id ulid.ULID) error {
	proj, err := pm.Get(ctx, id)
	if err != nil {
		return nil
	}
	if len(proj.ImageIds) != 0 {
		return &db.FieldError{"project", "ImageIds", "Can't delete non-empty project"}
	}
	if len(proj.DiskIds) != 0 {
		return &db.FieldError{"project", "DiskIds", "Can't delete non-empty project"}
	}
	c := pm.conn
	txn := c.NewTransaction()
	forfeitUniqueProjectName(proj, txn)
	txn.Delete(proj)
	return txn.Commit(ctx)
}

func (pm *etcdProjectManager) Watch(ctx context.Context) chan *db.Project {
	entityCh := pm.conn.watchEntity(ctx, func() db.Entity { return pm.NewEntity() })
	resultCh := make(chan *db.Project)
	go func() {
	loop:
		for {
			select {
			case entity := <-entityCh:
				if entity == nil {
					break loop
				}
				resultCh <- entity.(*db.Project)
			case <-ctx.Done():
				break loop
			}
		}
		close(resultCh)
	}()
	return resultCh
}
