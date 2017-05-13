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
	"fmt"
	"github.com/antonf/minicloud/db"
	"github.com/antonf/minicloud/utils"
	"github.com/oklog/ulid"
)

type Project struct {
	db.EntityHeader
	Name      string
	ImageIds  []ulid.ULID
	DiskIds   []ulid.ULID
	ServerIds []ulid.ULID
}

func (e *Project) String() string {
	return fmt.Sprintf("Project{Id:%s Name:%s [sv=%d cr=%d mr=%d]}", e.Id, e.Name, e.SchemaVersion, e.CreateRev, e.ModifyRev)
}
func (e *Project) EntityName() string {
	return "Project"
}
func (e *Project) Copy() *Project {
	return &Project{EntityHeader: e.EntityHeader, Name: e.Name, ImageIds: utils.ULIDListCopy(e.ImageIds), DiskIds: utils.ULIDListCopy(e.DiskIds), ServerIds: utils.ULIDListCopy(e.ServerIds)}
}

type Flavor struct {
	db.EntityHeader
	Name      string
	NumCPUs   int
	RAM       int
	ServerIds []ulid.ULID
}

func (e *Flavor) String() string {
	return fmt.Sprintf("Flavor{Id:%s Name:%s NumCPUs:%s RAM:%s [sv=%d cr=%d mr=%d]}", e.Id, e.Name, e.NumCPUs, e.RAM, e.SchemaVersion, e.CreateRev, e.ModifyRev)
}
func (e *Flavor) EntityName() string {
	return "Flavor"
}
func (e *Flavor) Copy() *Flavor {
	return &Flavor{EntityHeader: e.EntityHeader, Name: e.Name, NumCPUs: e.NumCPUs, RAM: e.RAM, ServerIds: utils.ULIDListCopy(e.ServerIds)}
}

type Image struct {
	db.EntityHeader
	Name      string
	Checksum  string
	ProjectId ulid.ULID
	DiskIds   []ulid.ULID
}

func (e *Image) String() string {
	return fmt.Sprintf("Image{Id:%s Name:%s [sv=%d cr=%d mr=%d]}", e.Id, e.Name, e.SchemaVersion, e.CreateRev, e.ModifyRev)
}
func (e *Image) EntityName() string {
	return "Image"
}
func (e *Image) Copy() *Image {
	return &Image{EntityHeader: e.EntityHeader, Name: e.Name, Checksum: e.Checksum, ProjectId: e.ProjectId, DiskIds: utils.ULIDListCopy(e.DiskIds)}
}

type Disk struct {
	db.EntityHeader
	ProjectId ulid.ULID
	ImageId   ulid.ULID
	Desc      string
	Pool      string
	Size      uint64
	ServerId  ulid.ULID
}

func (e *Disk) String() string {
	return fmt.Sprintf("Disk{Id:%s [sv=%d cr=%d mr=%d]}", e.Id, e.SchemaVersion, e.CreateRev, e.ModifyRev)
}
func (e *Disk) EntityName() string {
	return "Disk"
}
func (e *Disk) Copy() *Disk {
	return &Disk{EntityHeader: e.EntityHeader, ProjectId: e.ProjectId, ImageId: e.ImageId, Desc: e.Desc, Pool: e.Pool, Size: e.Size, ServerId: e.ServerId}
}

type Server struct {
	db.EntityHeader
	ProjectId ulid.ULID
	FlavorId  ulid.ULID
	DiskIds   []ulid.ULID
	Name      string
}

func (e *Server) String() string {
	return fmt.Sprintf("Server{Id:%s Name:%s [sv=%d cr=%d mr=%d]}", e.Id, e.Name, e.SchemaVersion, e.CreateRev, e.ModifyRev)
}
func (e *Server) EntityName() string {
	return "Server"
}
func (e *Server) Copy() *Server {
	return &Server{EntityHeader: e.EntityHeader, ProjectId: e.ProjectId, FlavorId: e.FlavorId, DiskIds: utils.ULIDListCopy(e.DiskIds), Name: e.Name}
}
