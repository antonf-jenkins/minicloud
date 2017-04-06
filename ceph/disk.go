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
	"github.com/ceph/go-ceph/rbd"
	"log"
)

func CreateEmptyDisk(pool, name string, size uint64) error {
	// Create connection; defer shutdown
	conn, err := NewConnection(pool)
	if err != nil {
		log.Printf("ceph: new connection failed: %s", err)
		return err
	}
	defer conn.Close()

	// Create image
	if _, err := rbd.Create(conn.ioctx[pool], name, size, OptDiskOrder.Value()); err != nil {
		log.Printf("ceph: create disk pool=%s name=%s: %s", pool, name, err)
		return err
	}

	return nil
}

func CreateDiskFromImage(diskPool, diskName, imagePool, imageName, snap string, size uint64) error {
	// Create connection; defer shutdown
	conn, err := NewConnection(imagePool, diskPool)
	if err != nil {
		log.Printf("ceph: new connection failed: %s", err)
		return err
	}
	defer conn.Close()

	// Clone image
	image := rbd.GetImage(conn.ioctx[imagePool], imageName)
	disk, err := image.Clone(snap, conn.ioctx[diskPool], diskName, RbdFeaturesDefault, OptDiskOrder.Value())
	if err != nil {
		log.Printf(
			"ceph: failed to clone image diskPool=%s diskName=%s imagePool=%s imageName=%s: %s",
			diskPool, diskName, imagePool, imageName, err)
		return err
	}

	// Resize result
	if err := disk.Open(); err != nil {
		log.Printf("ceph: open disk error pool=%s name=%s: %s", diskPool, diskName, err)
		return err
	}
	defer disk.Close()
	if err := disk.Resize(size); err != nil {
		log.Printf("ceph: failed to resize disk pool=%s name=%s : %s", diskPool, diskName, err)
		if removeErr := disk.Remove(); removeErr != nil {
			log.Printf("ceph: failed to remove disk pool=%s name=%s: %s", diskPool, diskName, removeErr)
		}
		return err
	}

	return nil
}
