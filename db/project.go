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
	"encoding/json"
	"fmt"
	"github.com/antonf/minicloud/fsm"
	"github.com/antonf/minicloud/utils"
	"github.com/oklog/ulid"
	"reflect"
	"regexp"
)

var regexpProjectName = regexp.MustCompile("[a-zA-Z0-9_.:-]{3,}")

type ProjectManager interface {
	NewEntity() *Project
	Get(ctx context.Context, id ulid.ULID) (*Project, error)
	Create(ctx context.Context, proj *Project) error
	Update(ctx context.Context, proj *Project, initiator fsm.Initiator) error
	Delete(ctx context.Context, id ulid.ULID) error
	Watch(ctx context.Context) chan *Project
}

type Project struct {
	EntityHeader
	Id       ulid.ULID
	Name     string
	ImageIds []ulid.ULID
	DiskIds  []ulid.ULID
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
	if len(p.DiskIds) > 0 {
		return &FieldError{"project", "DiskIds", "Should be empty"}
	}
	return nil
}

func (p *Project) validateUpdate(initiator fsm.Initiator) error {
	if err := checkFieldRegexp("project", "Name", p.Name, regexpProjectName); err != nil {
		return err
	}
	origVal := p.original.(*Project)
	if p.Id != origVal.Id {
		return &FieldError{"project", "Id", "Field is read-only"}
	}
	if initiator != fsm.System {
		if !reflect.DeepEqual(origVal.ImageIds, p.ImageIds) {
			return &FieldError{"project", "ImageIds", "Field is read-only"}
		}
		if !reflect.DeepEqual(origVal.DiskIds, p.DiskIds) {
			return &FieldError{"project", "DiskIds", "Field is read-only"}
		}
	}
	return nil
}

func (p *Project) claimUniqueName(txn Transaction) {
	txn.ClaimUnique(p, "name", p.Name)
}

func (p *Project) forfeitUniqueName(txn Transaction) {
	txn.ForfeitUnique(p, "name", p.Name)
}

type etcdProjectManager struct {
	conn *etcdConeection
}

func (pm *etcdProjectManager) NewEntity() *Project {
	return &Project{
		EntityHeader: EntityHeader{SchemaVersion: 1},
	}
}

func (pm *etcdProjectManager) Get(ctx context.Context, id ulid.ULID) (*Project, error) {
	proj := &Project{Id: id}
	if err := pm.conn.loadEntity(ctx, proj); err != nil {
		return nil, err
	}
	return proj, nil
}

func (pm *etcdProjectManager) Create(ctx context.Context, proj *Project) error {
	if err := proj.validate(); err != nil {
		return err
	}
	c := pm.conn
	proj.Id = utils.NewULID()
	txn := c.NewTransaction()
	txn.Create(proj)
	proj.claimUniqueName(txn)
	return txn.Commit(ctx)
}

func (pm *etcdProjectManager) Update(ctx context.Context, proj *Project, initiator fsm.Initiator) error {
	if err := proj.validateUpdate(initiator); err != nil {
		return err
	}
	c := pm.conn
	origProj := proj.original.(*Project)
	txn := c.NewTransaction()
	txn.Update(proj)
	if origProj.Name != proj.Name {
		origProj.forfeitUniqueName(txn)
		proj.claimUniqueName(txn)
	}
	return txn.Commit(ctx)
}

func (pm *etcdProjectManager) Delete(ctx context.Context, id ulid.ULID) error {
	proj, err := pm.Get(ctx, id)
	if err != nil {
		return nil
	}
	if len(proj.ImageIds) != 0 {
		return &FieldError{"project", "ImageIds", "Can't delete non-empty project"}
	}
	if len(proj.DiskIds) != 0 {
		return &FieldError{"project", "DiskIds", "Can't delete non-empty project"}
	}
	c := pm.conn
	txn := c.NewTransaction()
	proj.forfeitUniqueName(txn)
	txn.Delete(proj)
	return txn.Commit(ctx)
}

func (pm *etcdProjectManager) Watch(ctx context.Context) chan *Project {
	entityCh := pm.conn.watchEntity(ctx, func() Entity { return pm.NewEntity() })
	resultCh := make(chan *Project)
	go func() {
	loop:
		for {
			select {
			case entity := <-entityCh:
				if entity == nil {
					break loop
				}
				resultCh <- entity.(*Project)
			case <-ctx.Done():
				break loop
			}
		}
		close(resultCh)
	}()
	return resultCh
}
