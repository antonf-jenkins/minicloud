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
	"crypto/md5"
	"fmt"
	"github.com/antonf/minicloud/ceph"
	"github.com/antonf/minicloud/db"
	"github.com/antonf/minicloud/fsm"
	"github.com/oklog/ulid"
	"io"
	"net/http"
)

func setImageStateError(ctx context.Context, conn db.Connection, id ulid.ULID) error {
	image, err := conn.Images().Get(ctx, id)
	if err != nil {
		return err
	}
	image.State = fsm.StateError
	if err := conn.Images().Update(ctx, image, fsm.System); err != nil {
		return err
	}
	return nil
}

func UploadImage(ctx context.Context, conn db.Connection, w http.ResponseWriter, req *http.Request, params PathParams) {
	// Don't accept uploads without length specified
	if req.ContentLength <= 0 {
		w.WriteHeader(http.StatusLengthRequired)
		return
	}

	// Get image and check it's state
	id := params.GetULID("id")
	image, err := conn.Images().Get(ctx, id)
	if err != nil {
		writeError(w, err)
		return
	}

	image.State = fsm.StateUploading
	if err := conn.Images().Update(ctx, image, fsm.System); err != nil {
		writeError(w, err)
		return
	}

	// Create new image in images pool
	md5hash := md5.New()
	content := io.TeeReader(req.Body, md5hash)
	contentLength := uint64(req.ContentLength)
	if err := ceph.CreateImageWithContent("images", image.Id.String(), contentLength, content); err != nil {
		retry(func() error {
			return setImageStateError(ctx, conn, id)
		})
		writeError(w, err)
		return
	}

	// Update image state
	if err := retry(func() error {
		image, err := conn.Images().Get(ctx, id)
		if err != nil {
			return err
		}
		image.State = fsm.StateReady
		image.Checksum = fmt.Sprintf("%32x", md5hash.Sum(nil))
		if err := conn.Images().Update(ctx, image, fsm.System); err != nil {
			return err
		}
		return nil
	}); err != nil {
		retry(func() error {
			return setImageStateError(ctx, conn, id)
		})
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
