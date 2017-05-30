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
	"net/http"
	"os"
)

func main() {
	ctx := context.Background()
	log.Initialize(ctx)

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
	http.ListenAndServe("0.0.0.0:1959", apiServer)
}
