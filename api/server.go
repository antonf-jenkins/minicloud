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
package api

import (
	"github.com/antonf/minicloud/log"
	"github.com/antonf/minicloud/utils"
	"net/http"
	"strings"
)

type Server struct {
	root *MountPoint
}

func (api *Server) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	path := req.URL.Path
	for path != "" && path[0] == '/' {
		path = path[1:]
	}
	ctx := log.WithValues(req.Context(), "request_id", utils.NewULID())
	if !processRest(&packedParams{ctx, req, w}, api.root, path, nil) {
		Respond404(w)
	}
}

func (api *Server) MountPoint(path string) *MountPoint {
	curNode := api.root
	for elem, rest := nextPathElem(strings.Trim(path, "/")); elem != ""; elem, rest = nextPathElem(rest) {
		curNode = curNode.Child(elem)
	}
	return curNode
}

func NewServer() *Server {
	return &Server{root: newMountPoint("")}
}
