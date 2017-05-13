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
package model

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/antonf/minicloud/db"
	"github.com/antonf/minicloud/utils"
	"github.com/oklog/ulid"
	"regexp"
)

type ImageManager struct {
	conn db.Connection
}

func Images(conn db.Connection) *ImageManager {
	return &ImageManager{conn: conn}
}

var regexpImageName = regexp.MustCompile("^[a-zA-Z0-9_.:-]{3,200}$")

func (m *ImageManager) NewEntity() *Image {
	return &Image{EntityHeader: db.EntityHeader{SchemaVersion: 1, State: db.StateCreated}}
}
func (m *ImageManager) Get(ctx context.Context, id ulid.ULID) (*Image, error) {
	value, err := m.conn.RawRead(ctx, fmt.Sprintf("/minicloud/db/data/image/%s", id))
	if err != nil {
		return nil, err
	}
	if value.Data == nil {
		return nil, &db.NotFoundError{Entity: "Image", Id: id}
	}
	entity := &Image{}
	if err := json.Unmarshal(value.Data, entity); err != nil {
		return nil, err
	}
	entity.CreateRev = value.CreateRev
	entity.ModifyRev = value.ModifyRev
	entity.Original = entity.Copy()
	return entity, nil
}
func (m *ImageManager) Create(ctx context.Context, entity *Image, initiator db.Initiator) error {
	entity.Id = utils.NewULID()
	if err := ImageFSM.CheckInitialState(entity.State); err != nil {
		return err
	}
	if !regexpImageName.MatchString(entity.Name) {
		return &db.FieldError{Entity: "image", Field: "Name", Message: "Name should be between 3 and 200 characters from following set: a-z A-Z 0-9 _.:-"}
	}
	if entity.Checksum != "" {
		return &db.FieldError{Entity: "image", Field: "Checksum", Message: "Should be empty"}
	}
	if len(entity.DiskIds) != 0 {
		return &db.FieldError{Entity: "image", Field: "DiskIds", Message: "Should be empty"}
	}
	txn := m.conn.NewTransaction()
	txn.Create(ctx, entity)
	if project, err := Projects(m.conn).Get(ctx, entity.ProjectId); err != nil {
		return err
	} else {
		project.ImageIds = append(project.ImageIds, entity.Id)
		txn.Update(ctx, project)
	}
	key0 := fmt.Sprintf("/minicloud/db/meta/image/project/%s/name/%s", entity.ProjectId, entity.Name)
	txn.CreateMeta(ctx, key0, entity.Id.String())
	ImageFSM.Notify(ctx, txn, entity)
	return txn.Commit(ctx)
}
func (m *ImageManager) Update(ctx context.Context, entity *Image, initiator db.Initiator) error {
	origEntity := entity.Original.(*Image)
	if err := ImageFSM.CheckTransition(origEntity.State, entity.State, initiator); err != nil {
		return err
	}
	if !regexpImageName.MatchString(entity.Name) {
		return &db.FieldError{Entity: "image", Field: "Name", Message: "Name should be between 3 and 200 characters from following set: a-z A-Z 0-9 _.:-"}
	}
	if initiator != db.InitiatorSystem && entity.Checksum != origEntity.Checksum {
		return &db.FieldError{Entity: "image", Field: "Checksum", Message: "Field change prohibited"}
	}
	if entity.ProjectId != origEntity.ProjectId {
		return &db.FieldError{Entity: "image", Field: "ProjectId", Message: "Field change prohibited"}
	}
	if !utils.ULIDSliceEqual(entity.DiskIds, origEntity.DiskIds) {
		return &db.FieldError{Entity: "image", Field: "DiskIds", Message: "Field change prohibited"}
	}
	txn := m.conn.NewTransaction()
	txn.Update(ctx, entity)
	if entity.ProjectId != origEntity.ProjectId || entity.Name != origEntity.Name {
		forfeitKey0 := fmt.Sprintf("/minicloud/db/meta/image/project/%s/name/%s", origEntity.ProjectId, origEntity.Name)
		txn.CheckMeta(ctx, forfeitKey0, origEntity.Id.String())
		txn.DeleteMeta(ctx, forfeitKey0)
		claimKey0 := fmt.Sprintf("/minicloud/db/meta/image/project/%s/name/%s", entity.ProjectId, entity.Name)
		txn.CreateMeta(ctx, claimKey0, entity.Id.String())
	}
	ImageFSM.Notify(ctx, txn, entity)
	return txn.Commit(ctx)
}
func (m *ImageManager) IntentDelete(ctx context.Context, id ulid.ULID, initiator db.Initiator) error {
	entity, err := Images(m.conn).Get(ctx, id)
	if err != nil {
		return err
	}
	if err := ImageFSM.CheckTransition(entity.State, db.StateDeleting, initiator); err != nil {
		return err
	}
	if len(entity.DiskIds) != 0 {
		return &db.FieldError{Entity: "image", Field: "DiskIds", Message: "Should be empty"}
	}
	entity.State = db.StateDeleting
	txn := m.conn.NewTransaction()
	txn.Update(ctx, entity)
	ImageFSM.Notify(ctx, txn, entity)
	return txn.Commit(ctx)
}
func (m *ImageManager) Delete(ctx context.Context, id ulid.ULID, initiator db.Initiator) error {
	entity, err := Images(m.conn).Get(ctx, id)
	if err != nil {
		return err
	}
	if err := ImageFSM.CheckTransition(entity.State, db.StateDeleted, initiator); err != nil {
		return err
	}
	if len(entity.DiskIds) != 0 {
		return &db.FieldError{Entity: "image", Field: "DiskIds", Message: "Should be empty"}
	}
	txn := m.conn.NewTransaction()
	txn.Delete(ctx, entity)
	if project, err := Projects(m.conn).Get(ctx, entity.ProjectId); err != nil {
		return err
	} else {
		project.ImageIds = utils.RemoveULID(project.ImageIds, entity.Id)
		txn.Update(ctx, project)
	}
	key0 := fmt.Sprintf("/minicloud/db/meta/image/project/%s/name/%s", entity.ProjectId, entity.Name)
	txn.CheckMeta(ctx, key0, entity.Id.String())
	txn.DeleteMeta(ctx, key0)
	ImageFSM.DeleteNotification(ctx, txn, entity)
	return txn.Commit(ctx)
}
