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
	"github.com/oklog/ulid"
)

type State string
type Initiator int

const (
	InitiatorSystem Initiator = 1 << 0
	InitiatorUser   Initiator = 1 << 1
	DataPrefix                = "/minicloud/db/data"
	MetaPrefix                = "/minicloud/db/meta"
)

type RawValue struct {
	CreateRev, ModifyRev int64
	Key                  string
	Data                 []byte
}

type Connection interface {
	RawRead(ctx context.Context, key string) (*RawValue, error)
	RawReadPrefix(ctx context.Context, key string) ([]RawValue, error)
	RawWatchPrefix(ctx context.Context, prefix string) chan *RawValue
	NewTransaction() Transaction

	Projects() ProjectManager
	Images() ImageManager
	Disks() DiskManager
	Flavors() FlavorManager
}

type Transaction interface {
	Commit(ctx context.Context) error
	Create(ctx context.Context, entity Entity)
	Update(ctx context.Context, entity Entity)
	Delete(ctx context.Context, entity Entity)
	CreateMeta(ctx context.Context, key, content string)
	CheckMeta(ctx context.Context, key, content string)
	DeleteMeta(ctx context.Context, key string)
	AcquireLock(ctx context.Context, key string)
	ReleaseLock(ctx context.Context, key string)
}

type ProjectManager interface {
	NewEntity() *Project
	Get(ctx context.Context, id ulid.ULID) (*Project, error)
	Create(ctx context.Context, proj *Project) error
	Update(ctx context.Context, proj *Project, initiator Initiator) error
	IntentDelete(ctx context.Context, id ulid.ULID, initiator Initiator) error
}

type ImageManager interface {
	NewEntity() *Image
	Get(ctx context.Context, id ulid.ULID) (*Image, error)
	Create(ctx context.Context, img *Image) error
	Update(ctx context.Context, img *Image, initiator Initiator) error
	IntentDelete(ctx context.Context, id ulid.ULID, initiator Initiator) error
	Delete(ctx context.Context, id ulid.ULID, initiator Initiator) error
}

type DiskManager interface {
	NewEntity() *Disk
	Get(ctx context.Context, id ulid.ULID) (*Disk, error)
	Create(ctx context.Context, disk *Disk) error
	Update(ctx context.Context, disk *Disk, initiator Initiator) error
	IntentDelete(ctx context.Context, id ulid.ULID, initiator Initiator) error
	Delete(ctx context.Context, id ulid.ULID, initiator Initiator) error
}

type FlavorManager interface {
	NewEntity() *Flavor
	List(ctx context.Context) ([]*Flavor, error)
	Get(ctx context.Context, id ulid.ULID) (*Flavor, error)
	Create(ctx context.Context, flavor *Flavor) error
	Update(ctx context.Context, flavor *Flavor, initiator Initiator) error
	IntentDelete(ctx context.Context, id ulid.ULID, initiator Initiator) error
}

type Project struct {
	EntityHeader
	Name     string
	ImageIds []ulid.ULID
	DiskIds  []ulid.ULID
}

func (p Project) String() string {
	return fmt.Sprintf(
		"Project{Id:%s Name:%s [sv=%d cr=%d mr=%d]}",
		p.Id, p.Name, p.SchemaVersion, p.CreateRev, p.ModifyRev)
}

type Image struct {
	EntityHeader
	ProjectId ulid.ULID
	Name      string
	Checksum  string
	DiskIds   []ulid.ULID
}

func (img *Image) String() string {
	return fmt.Sprintf(
		"Image{Id:%s Name:%s [sv=%d cr=%d mr=%d]}",
		img.Id, img.Name, img.SchemaVersion, img.CreateRev, img.ModifyRev)
}

type Disk struct {
	EntityHeader
	ProjectId ulid.ULID
	Desc      string
	Pool      string
	ImageId   ulid.ULID
	Size      uint64
}

func (disk *Disk) String() string {
	return fmt.Sprintf(
		"Disk{Id:%s [sv=%d cr=%d mr=%d]}",
		disk.Id, disk.SchemaVersion, disk.CreateRev, disk.ModifyRev)
}

type Flavor struct {
	EntityHeader
	Name       string
	NumCPUs    int
	RAM        int
	MachineIds []ulid.ULID
}

func (flavor *Flavor) String() string {
	return fmt.Sprintf(
		"Flavor{Id:%s Name:%s NumCPUs:%d RAM:%dMb [sv=%d cr=%d mr=%d]}",
		flavor.Id, flavor.Name, flavor.NumCPUs, flavor.RAM,
		flavor.SchemaVersion, flavor.CreateRev, flavor.ModifyRev)
}
