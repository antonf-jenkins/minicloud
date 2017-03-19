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
package qemu

import (
	"github.com/oklog/ulid"
	"os"
	"os/exec"
)

type DiskCache string

const (
	CacheWriteThrough DiskCache = "writethrough"
	CacheWriteBack    DiskCache = "writeback"
	CacheNone         DiskCache = "none"
	CacheUnsafe       DiskCache = "unsafe"
	CacheDirectSync   DiskCache = "directsync"
)

type NetworkDevice struct {
	InterfaceName string
	MacAddress    string
}

type StorageDevice struct {
	Pool  string
	Disk  string
	Cache DiskCache
}

type VirtualMachine struct {
	Id       ulid.ULID
	VncPort  int
	Cpu      string
	Root     string
	Nics     []NetworkDevice
	Disks    []StorageDevice
	MemLock  bool
	VhostNet bool

	cmd   *exec.Cmd
	files []*os.File
	mon   *Monitor
}

func (vm *VirtualMachine) Monitor() *Monitor {
	if vm.mon == nil {
		panic("Monitor() call on uninitialized VM")
	}
	return vm.mon
}
