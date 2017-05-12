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

type Project struct {
	EntityHeader
	Name      string
	ImageIds  []ulid.ULID
	DiskIds   []ulid.ULID
	ServerIds []ulid.ULID
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
	ServerId ulid.ULID
}

func (disk *Disk) String() string {
	return fmt.Sprintf(
		"Disk{Id:%s [sv=%d cr=%d mr=%d]}",
		disk.Id, disk.SchemaVersion, disk.CreateRev, disk.ModifyRev)
}

type Flavor struct {
	EntityHeader
	Name      string
	NumCPUs   int
	RAM       int
	ServerIds []ulid.ULID
}

func (flavor *Flavor) String() string {
	return fmt.Sprintf(
		"Flavor{Id:%s Name:%s NumCPUs:%d RAM:%dMb [sv=%d cr=%d mr=%d]}",
		flavor.Id, flavor.Name, flavor.NumCPUs, flavor.RAM,
		flavor.SchemaVersion, flavor.CreateRev, flavor.ModifyRev)
}

type Server struct {
	EntityHeader
	ProjectId ulid.ULID
	FlavorId  ulid.ULID
	Name      string
	DiskIds   []ulid.ULID
	//PortIds   []ulid.ULID
}

func (m *Server) String() string {
	return fmt.Sprintf(
		"Server{Id:%s Name:%s [sv=%d cr=%d mr=%d]}",
		m.Id, m.Name, m.SchemaVersion, m.CreateRev, m.ModifyRev)
}
