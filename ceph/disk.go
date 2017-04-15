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
)

func CreateEmptyDisk(ctx context.Context, pool, name string, size uint64) error {
	opCtx := log.WithValues(ctx, "pool", pool, "name", name, "size", size)

	// Create connection; defer shutdown
	conn, err := NewConnection(ctx, pool)
	if err != nil {
		return err
	}
	defer conn.Close()

	// Create image
	if _, err := rbd.Create(conn.ioctx[pool], name, size, OptDiskOrder.Value()); err != nil {
		logger.Error(opCtx, "error creating disk", "error", err)
		return err
	}

	logger.Info(opCtx, "created empty disk")
	return nil
}

func CreateDiskFromImage(ctx context.Context, diskPool, diskName, imagePool, imageName, snap string, size uint64) error {
	opCtx := log.WithValues(ctx,
		"disk_pool", diskPool, "disk_name", diskName,
		"image_pool", imagePool, "image_name", imageName,
		"size", size)

	// Create connection; defer shutdown
	conn, err := NewConnection(ctx, imagePool, diskPool)
	if err != nil {
		return err
	}
	defer conn.Close()

	// Clone image
	image := rbd.GetImage(conn.ioctx[imagePool], imageName)
	disk, err := image.Clone(snap, conn.ioctx[diskPool], diskName, RbdFeaturesDefault, OptDiskOrder.Value())
	if err != nil {
		logger.Error(opCtx, "failed to clone image", "error", err)
		return err
	}

	// Resize result
	if err := disk.Open(); err != nil {
		logger.Error(opCtx, "failed to open disk", "error", err)
		return err
	}
	defer disk.Close()
	if err := disk.Resize(size); err != nil {
		logger.Error(opCtx, "failed to resize disk", "error", err)
		if removeErr := disk.Remove(); removeErr != nil {
			logger.Error(opCtx, "failed to remove disk after failure", "error", removeErr)
		}
		return err
	}

	logger.Info(opCtx, "created disk from image")
	return nil
}

func ResizeDisk(ctx context.Context, pool, name string, size uint64) error {
	opCtx := log.WithValues(ctx, "pool", pool, "name", name, "size", size)

	// Create connection; defer shutdown
	conn, err := NewConnection(ctx, pool)
	if err != nil {
		return err
	}
	defer conn.Close()

	// Open disk
	disk := rbd.GetImage(conn.ioctx[pool], name)
	if err := disk.Open(); err != nil {
		logger.Error(opCtx, "failed to open disk", "error", err)
		return err
	}
	defer disk.Close()

	// Check old size
	oldSize, err := disk.GetSize()
	if err != nil {
		logger.Error(opCtx, "failed to get disk size", "error", err)
		return err
	}
	defer disk.Close()
	if oldSize == size {
		logger.Info(opCtx, "size not changed", "old_size", oldSize)
		return nil
	}

	// Resize disk
	if err := disk.Resize(size); err != nil {
		logger.Error(opCtx, "failed to resize disk", "error", err)
		return err
	}

	logger.Info(opCtx, "disk resized")
	return nil
}

func DeleteDisk(ctx context.Context, pool, name string) error {
	opCtx := log.WithValues(ctx, "pool", pool, "name", name)

	// Create connection; defer shutdown
	conn, err := NewConnection(ctx, pool)
	if err != nil {
		return err
	}
	defer conn.Close()

	disk := rbd.GetImage(conn.ioctx[pool], name)
	if err = disk.Remove(); err != nil && err != rbd.RbdErrorNotFound {
		logger.Error(opCtx, "failed to delete disk", "error", err)
		return err
	}

	if err == rbd.RbdErrorNotFound {
		logger.Info(opCtx, "disk didn't exist")
	} else {
		logger.Info(opCtx, "disk removed")
	}
	return nil
}
