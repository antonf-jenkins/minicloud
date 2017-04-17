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

var regexpFlavorName = regexp.MustCompile("^[a-z0-9_.-]{3,}$")

func validateFlavorEmpty(p *db.Flavor, message string) error {
	if len(p.MachineIds) > 0 {
		return &db.FieldError{Entity: "flavor", Field: "MachineIds", Message: message}
	}
	return nil
}

func validateFlavor(flavor *db.Flavor) error {
	if err := checkFieldRegexp("flavor", "Name", flavor.Name, regexpFlavorName); err != nil {
		return err
	}
	if flavor.NumCPUs <= 0 {
		return &db.FieldError{Entity: "flavor", Field: "NumCPUs", Message: "Should be more than 0"}
	}
	if flavor.RAM <= 0 {
		return &db.FieldError{Entity: "flavor", Field: "RAM", Message: "Should be more than 0"}
	}
	if err := fsm.FlavorFSM.CheckInitialState(flavor.State); err != nil {
		return err
	}
	if err := validateFlavorEmpty(flavor, "Should be empty"); err != nil {
		return err
	}
	return nil
}

func validateUpdateFlavor(flavor *db.Flavor, initiator db.Initiator) error {
	if err := checkFieldRegexp("flavor", "Name", flavor.Name, regexpFlavorName); err != nil {
		return err
	}
	origFlavor := flavor.Original.(*db.Flavor)
	if flavor.Id != origFlavor.Id {
		return &db.FieldError{Entity: "flavor", Field: "Id", Message: "Field is read-only"}
	}
	if flavor.NumCPUs != origFlavor.NumCPUs {
		return &db.FieldError{Entity: "flavor", Field: "NumCPUs", Message: "Field is read-only"}
	}
	if flavor.RAM != origFlavor.RAM {
		return &db.FieldError{Entity: "flavor", Field: "RAM", Message: "Field is read-only"}
	}
	if err := fsm.FlavorFSM.CheckTransition(origFlavor.State, flavor.State, initiator); err != nil {
		return err
	}
	if initiator != db.InitiatorSystem {
		if !reflect.DeepEqual(origFlavor.MachineIds, flavor.MachineIds) {
			return &db.FieldError{Entity: "flavor", Field: "MachineIds", Message: "Field is read-only"}
		}
	}
	return nil
}

func claimUniqueFlavorName(ctx context.Context, flavor *db.Flavor, txn db.Transaction) {
	txn.CreateMeta(ctx, uniqueMetaKey(flavor, "name", flavor.Name), flavor.Id.String())
}

func forfeitUniqueFlavorName(ctx context.Context, flavor *db.Flavor, txn db.Transaction) {
	key := uniqueMetaKey(flavor, "name", flavor.Name)
	txn.CheckMeta(ctx, key, flavor.Id.String())
	txn.DeleteMeta(ctx, key)
}

type etcdFlavorManager struct {
	conn *etcdConnection
}

func (fm *etcdFlavorManager) NewEntity() *db.Flavor {
	return &db.Flavor{
		EntityHeader: db.EntityHeader{
			SchemaVersion: 1,
			State:         db.StateReady,
		},
	}
}

func (fm *etcdFlavorManager) List(ctx context.Context) ([]*db.Flavor, error) {
	list, err := fm.conn.listEntities(ctx, "flavor", func() db.Entity { return fm.NewEntity() })
	if err != nil {
		return nil, err
	}
	result := make([]*db.Flavor, len(list))
	for index, entity := range list {
		result[index] = entity.(*db.Flavor)
	}
	return result, nil
}

func (fm *etcdFlavorManager) Get(ctx context.Context, id ulid.ULID) (*db.Flavor, error) {
	flavor := &db.Flavor{EntityHeader: db.EntityHeader{Id: id}}
	if err := fm.conn.loadEntity(ctx, flavor); err != nil {
		return nil, err
	}
	return flavor, nil
}

func (fm *etcdFlavorManager) Create(ctx context.Context, flavor *db.Flavor) error {
	if err := validateFlavor(flavor); err != nil {
		return err
	}
	flavor.Id = utils.NewULID()
	txn := fm.conn.NewTransaction()
	txn.Create(ctx, flavor)
	claimUniqueFlavorName(ctx, flavor, txn)
	return txn.Commit(ctx)
}

func (fm *etcdFlavorManager) Update(ctx context.Context, flavor *db.Flavor, initiator db.Initiator) error {
	if err := validateUpdateFlavor(flavor, initiator); err != nil {
		return err
	}
	origFlavor := flavor.Original.(*db.Flavor)
	txn := fm.conn.NewTransaction()
	txn.Update(ctx, flavor)
	if origFlavor.Name != flavor.Name {
		forfeitUniqueFlavorName(ctx, origFlavor, txn)
		claimUniqueFlavorName(ctx, flavor, txn)
	}
	return txn.Commit(ctx)
}

func (fm *etcdFlavorManager) Delete(ctx context.Context, id ulid.ULID, initiator db.Initiator) error {
	flavor, err := fm.Get(ctx, id)
	if err != nil {
		return err
	}
	if err := validateFlavorEmpty(flavor, "Can't delete referenced flavor"); err != nil {
		return err
	}
	if err := fsm.FlavorFSM.CheckTransition(flavor.State, db.StateDeleted, initiator); err != nil {
		return err
	}
	txn := fm.conn.NewTransaction()
	forfeitUniqueFlavorName(ctx, flavor, txn)
	txn.Delete(ctx, flavor)
	return txn.Commit(ctx)
}
