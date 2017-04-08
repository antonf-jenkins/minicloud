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
package ceph

import (
	"context"
	"github.com/antonf/minicloud/log"
	"github.com/ceph/go-ceph/rbd"
	"io"
)

func CreateImageWithContent(ctx context.Context, pool, name string, size uint64, reader io.Reader) error {
	opCtx := log.WithValues(ctx, "pool", pool, "name", name, "size", size)
	// Create connection; defer shutdown
	conn, err := NewConnection(ctx, pool)
	if err != nil {
		return err
	}
	defer conn.Close()

	// Create and write the image
	img, err := rbd.Create(conn.ioctx[pool], name, size, OptImageOrder.Value())
	if err != nil {
		logger.Error(opCtx, "failed to create image", "error", err)
		return err
	}

	// Create and write data to image
	needImageRemove := false
	if err := img.Open(); err != nil {
		logger.Error(opCtx, "failed to open image", "error", err)
		return err
	}
	defer func() {
		img.Close()
		if needImageRemove {
			if err := img.Remove(); err != nil {
				logger.Error(opCtx, "failed to cleanup image", "error", err)
			}
		}
	}()
	if uploadedSize, err := io.Copy(img, reader); err != nil {
		needImageRemove = true
		logger.Error(opCtx, "image upload error", "error", err)
		return err
	} else {
		logger.Info(opCtx, "image uploaded", "size", uploadedSize)
	}
	if err := img.Flush(); err != nil {
		needImageRemove = true
		logger.Error(opCtx, "image flush error", "error", err)
		return err
	}

	// Create and protect snapshot
	snap, err := img.CreateSnapshot("base")
	if err != nil {
		needImageRemove = true
		logger.Error(opCtx, "failed to create snapshot", "error", err)
		return err
	}
	if err := snap.Protect(); err != nil {
		if snapErr := snap.Remove(); snapErr != nil {
			logger.Error(opCtx, "failed to cleanup snapshot", "error", snapErr)
		} else {
			needImageRemove = true
		}
		return err
	}

	logger.Info(opCtx, "created image")
	return nil
}

func DeleteImage(ctx context.Context, pool, name string) error {
	opCtx := log.WithValues(ctx, "pool", pool, "name", name)

	// Create connection; defer shutdown
	conn, err := NewConnection(ctx)
	if err != nil {
		return err
	}
	defer conn.Close()

	img := rbd.GetImage(conn.ioctx[pool], name)
	snapNames, err := img.GetSnapshotNames()
	if err != nil {
		logger.Error(opCtx, "failed to get snapshot list", "error", err)
		return err
	}

	// Remove all snapshots, probably unprotecting them
	for _, snapName := range snapNames {
		snap := img.GetSnapshot(snapName.Name)
		isProtected, err := snap.IsProtected()
		if err != nil {
			logger.Error(opCtx, "failed to get protected flag", "snapshot", snapName, "error", err)
			return err
		}
		if isProtected {
			if err := snap.Unprotect(); err != nil {
				logger.Error(opCtx, "failed to unprotect", "snapshot", snapName, "error", err)
				return err
			}
		}
		if err := snap.Remove(); err != nil {
			logger.Error(opCtx, "failed to remove snapshot", "snapshot", snapName, "error", err)
			return err
		}
	}

	// Remove the image
	if err := img.Remove(); err != nil {
		logger.Error(opCtx, "failed to remove image", "error", err)
		return err
	}

	logger.Info(opCtx, "removed image")
	return nil
}
