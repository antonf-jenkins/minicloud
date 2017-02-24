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
	"net/http"
	"strings"
)

func Respond404(w http.ResponseWriter) {
	w.Header().Add(HeaderContentType, ContentTypePlaintext)
	w.WriteHeader(http.StatusNotFound)
	w.Write([]byte("Resourse don't exists\n"))
}

func Respond405(w http.ResponseWriter, methods []string) {
	w.Header().Add(HeaderAllowed, strings.Join(methods, ", "))
	w.WriteHeader(http.StatusMethodNotAllowed)
}

func nextPathElem(path string) (string, string) {
	idx := strings.Index(path, "/")
	if idx == -1 {
		return path, ""
	}
	elem := path[0:idx]
	rest := path[idx+1:]
	for rest != "" && rest[0] == '/' {
		rest = rest[1:]
	}
	return elem, rest
}

func nextLen(chain *link) int {
	if chain == nil {
		return 1
	} else {
		return chain.len + 1
	}
}
