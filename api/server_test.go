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
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func BenchmarkProcess(b *testing.B) {
	api := NewServer()
	handler := func(ctx context.Context, w http.ResponseWriter, req *http.Request, params PathParams) {
		params.GetULID("id")
		params.GetString("foo")
	}
	api.MountPoint("/bar/{id:ulid}/{foo:string}").Mount("GET", handler)
	req := httptest.NewRequest("GET", "/bar/01B984TSNZSVK7VX6STPAE95D0/vasya", nil)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		api.ServeHTTP(nil, req)
	}
}
