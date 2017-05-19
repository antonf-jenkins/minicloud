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
package main

import (
	"context"
	"github.com/antonf/minicloud/api"
	"github.com/antonf/minicloud/config"
	"github.com/antonf/minicloud/db/dbimpl"
	"github.com/antonf/minicloud/log"
	"github.com/antonf/minicloud/model"
	"github.com/antonf/minicloud/qemu"
	"github.com/antonf/minicloud/utils"
	"net/http"
	"os"
	"time"
)

func main() {
	ctx := context.Background()
	log.Initialize(ctx)
	logger := log.New("main")

	conn, err := dbimpl.NewConnection(ctx, 1)
	if err != nil {
		os.Exit(1)
		return
	}
	config.InitOptions(ctx, conn)
	err = model.WatchNotifications(ctx, conn)
	if err != nil {
		os.Exit(1)
		return
	}

	apiServer := api.NewServer()
	apiServer.MountPoint("/projects").MountManager(model.Projects(conn))
	apiServer.MountPoint("/images").MountManager(model.Images(conn))
	apiServer.MountPoint("/disks").MountManager(model.Disks(conn))
	apiServer.MountPoint("/images/{id:ulid}/contents").Mount(
		"PUT", func(ctx context.Context, w http.ResponseWriter, req *http.Request, params api.PathParams) {
			api.UploadImage(ctx, conn, w, req, params)
		})
	apiServer.MountPoint("/flavors").MountManager(model.Flavors(conn))
	apiServer.MountPoint("/servers").MountManager(model.Servers(conn))
	http.ListenAndServe("127.0.0.1:1959", apiServer)
	os.Exit(0)

	x := qemu.VirtualMachine{
		Id:       utils.NewULID(),
		Cpu:      "host",
		MemLock:  true,
		VhostNet: true,
		Disks: []qemu.StorageDevice{
			{Pool: "root", Disk: "test1", Cache: qemu.CacheWriteBack},
		},
		NICs: []qemu.NetworkDevice{
			{MacAddress: "52:54:00:1a:5d:f7", InterfaceName: "tap1a5df7"},
		},
		Root:    "/home/anton/vm",
		VncPort: 0,
	}

	if err := x.Start(ctx); err != nil {
		logger.Fatal(ctx, "_", "error", err)
	}

	for i := 1; i < 5; i++ {
		if err := x.Monitor().Cont(ctx); err != nil {
			logger.Fatal(ctx, "_", "error", err)
		}
		time.Sleep(30 * time.Second)
		if err := x.Monitor().Stop(ctx); err != nil {
			logger.Fatal(ctx, "_", "error", err)
		}
		time.Sleep(30 * time.Second)
	}

	if err := x.Monitor().Quit(ctx); err != nil {
		logger.Fatal(ctx, "_", "error", err)
	}

	if err := x.Wait(); err != nil {
		logger.Fatal(ctx, "Error while waiting", "error", err)
	}
}
