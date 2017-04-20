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
	"regexp"
)

var regexpProjectName = regexp.MustCompile("^[a-zA-Z0-9_.:-]{3,}$")

func validateProjectEmpty(p *db.Project, message string) error {
	if len(p.ImageIds) > 0 {
		return &db.FieldError{Entity: "project", Field: "ImageIds", Message: message}
	}
	if len(p.DiskIds) > 0 {
		return &db.FieldError{Entity: "project", Field: "DiskIds", Message: message}
	}
	if len(p.ServerIds) > 0 {
		return &db.FieldError{Entity: "project", Field: "ServerIds", Message: message}
	}
	return nil
}

func validateProject(p *db.Project) error {
	if err := checkFieldRegexp("project", "Name", p.Name, regexpProjectName); err != nil {
		return err
	}
	if err := validateProjectEmpty(p, "Should be empty"); err != nil {
		return err
	}
	return nil
}

func validateUpdateProject(p *db.Project, initiator db.Initiator) error {
	if err := checkFieldRegexp("project", "Name", p.Name, regexpProjectName); err != nil {
		return err
	}
	if err := checkReadOnlyFields(p, "Id", "ImageIds", "DiskIds", "ServerIds"); err != nil {
		return err
	}
	return nil
}

func claimUniqueProjectName(ctx context.Context, p *db.Project, txn db.Transaction) {
	txn.CreateMeta(ctx, uniqueMetaKey(p, "name", p.Name), p.Id.String())
}

func forfeitUniqueProjectName(ctx context.Context, p *db.Project, txn db.Transaction) {
	key := uniqueMetaKey(p, "name", p.Name)
	txn.CheckMeta(ctx, key, p.Id.String())
	txn.DeleteMeta(ctx, key)
}

type etcdProjectManager struct {
	conn *etcdConnection
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
	proj.Id = utils.NewULID()
	txn := pm.conn.NewTransaction()
	txn.Create(ctx, proj)
	claimUniqueProjectName(ctx, proj, txn)
	return txn.Commit(ctx)
}

func (pm *etcdProjectManager) Update(ctx context.Context, proj *db.Project, initiator db.Initiator) error {
	if err := validateUpdateProject(proj, initiator); err != nil {
		return err
	}
	origProj := proj.Original.(*db.Project)
	txn := pm.conn.NewTransaction()
	txn.Update(ctx, proj)
	if origProj.Name != proj.Name {
		forfeitUniqueProjectName(ctx, origProj, txn)
		claimUniqueProjectName(ctx, proj, txn)
	}
	return txn.Commit(ctx)
}

func (pm *etcdProjectManager) Delete(ctx context.Context, id ulid.ULID, initiator db.Initiator) error {
	proj, err := pm.Get(ctx, id)
	if err != nil {
		return err
	}
	if err := validateProjectEmpty(proj, "Can't delete non-empty project"); err != nil {
		return err
	}
	txn := pm.conn.NewTransaction()
	forfeitUniqueProjectName(ctx, proj, txn)
	txn.Delete(ctx, proj)
	return txn.Commit(ctx)
}
