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
	"fmt"
	"github.com/antonf/minicloud/db"
	"github.com/antonf/minicloud/qemu"
	"github.com/antonf/minicloud/utils"
	"github.com/oklog/ulid"
	"os"
)

var (
	ServerFSM       *StateMachine
	virtualMachines = make(map[ulid.ULID]*qemu.VirtualMachine)
)

func init() {
	ServerFSM = NewStateMachine().
		InitialState(db.StateCreated).
		UserTransition(db.StateReady, db.StateDeleting).
		UserTransition(db.StateError, db.StateDeleting).
		SystemTransition(db.StateCreated, db.StateReady).
		SystemTransition(db.StateCreated, db.StateError).
		SystemTransition(db.StateDeleting, db.StateDeleted).
		Hook(db.StateCreated, HandleServerCreated).
		Hook(db.StateDeleting, HandleServerDeleting)
}

func HandleServerCreated(ctx context.Context, conn db.Connection, entity db.Entity) {
	server := entity.(*Server)

	flavor, err := Flavors(conn).Get(ctx, server.FlavorId)
	if err != nil {
		failServerHandling(ctx, conn, server, err)
		return
	}

	storageDevices := make([]qemu.StorageDevice, len(server.DiskIds))
	for idx, diskId := range server.DiskIds {
		disk, err := Disks(conn).Get(ctx, diskId)
		if err != nil {
			failServerHandling(ctx, conn, server, err)
			return
		}
		device := &storageDevices[idx]
		device.Disk = disk.Id.String()
		device.Pool = disk.Pool
		device.Cache = qemu.CacheWriteBack
	}

	netDevices := []qemu.NetworkDevice{
	//		{MacAddress: randomMac(server.Id), InterfaceName: tapNameFromId(server.Id)},
	}

	root := "/home/anton/vm/" + server.Id.String() // TODO: option
	if err := os.MkdirAll(root, 0700); err != nil {
		failServerHandling(ctx, conn, server, err)
		return
	}

	vm := &qemu.VirtualMachine{
		Id:       server.Id,
		Cpu:      "host",
		MemLock:  false, // TODO: option
		VhostNet: false, // TODO: option
		Disks:    storageDevices,
		NICs:     netDevices,
		Root:     root, // TODO: option
		VncPort:  0,    // TODO: port allocation
		NumCPUs:  flavor.NumCPUs,
		RAM:      flavor.RAM,
	}

	if err := vm.Start(ctx); err != nil {
		failServerHandling(ctx, conn, server, err)
		return
	}
	go vm.Wait()

	if err := vm.Monitor().Cont(ctx); err != nil {
		vm.Kill(ctx)
		failServerHandling(ctx, conn, server, err)
		return
	}

	virtualMachines[server.Id] = vm
	utils.Retry(ctx, func(ctx context.Context) error {
		server, err := Servers(conn).Get(ctx, server.Id)
		if err != nil {
			return err
		}
		server.State = db.StateReady
		return Servers(conn).Update(ctx, server, db.InitiatorSystem)
	})
}

func HandleServerDeleting(ctx context.Context, conn db.Connection, entity db.Entity) {
	server := entity.(*Server)

	if vm, ok := virtualMachines[server.Id]; ok {
		delete(virtualMachines, server.Id)
		if err := vm.Monitor().Quit(ctx); err != nil {
			logger.Error(ctx, "failed to turn virtual machine off", "error", err)
			vm.Kill(ctx)
		}
		vm.Monitor().Close()
	}

	if err := Servers(conn).Delete(ctx, server.Id, db.InitiatorSystem); err != nil {
		logger.Error(ctx, "failed to delete server", "error", err)
	}
}

func tapNameFromId(id ulid.ULID) string {
	idStr := id.String()
	return "tap" + idStr[len(idStr)-12:]
}

func failServerHandling(ctx context.Context, conn db.Connection, server *Server, err error) {
	logger.Error(ctx, "state handling failed", "state", server.State, "error", err)
	utils.Retry(ctx, func(ctx context.Context) error {
		server, err := Servers(conn).Get(ctx, server.Id)
		if err != nil {
			return err
		}
		server.State = db.StateError
		return Servers(conn).Update(ctx, server, db.InitiatorSystem)
	})
}

func randomMac(id ulid.ULID) string {
	return fmt.Sprintf("52:54:00:%02x:%02x:%02x", id[15], id[14], id[13])
}
