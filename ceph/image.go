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
	"io"
	"log"
)

func CreateImageWithContent(pool, name string, size uint64, reader io.Reader) error {
	// Create connection; defer shutdown
	conn, err := NewConnection(pool)
	if err != nil {
		log.Printf("ceph: new connection failed: %s", err)
		return err
	}
	defer conn.Close()

	// Create and write the image
	img, err := rbd.Create(conn.ioctx[pool], name, size, OptImageOrder.Value())
	if err != nil {
		log.Printf("ceph: create image pool=%s name=%s: %s", pool, name, err)
		return err
	}

	// Create and write data to image
	needImageRemove := false
	if err := img.Open(); err != nil {
		log.Printf("ceph: open image error pool=%s name=%s: %s", pool, name, err)
		return err
	}
	defer func() {
		img.Close()
		if needImageRemove {
			if err := img.Remove(); err != nil {
				log.Printf("ceph: error removing image: %s", err)
			}
		}
	}()
	if uploadedSize, err := io.Copy(img, reader); err != nil {
		needImageRemove = true
		log.Printf("ceph: image upload error pool=%s name=%s: %s", pool, name, err)
		return err
	} else {
		log.Printf("ceph: image uploaded bytes=%d", uploadedSize)
	}
	if err := img.Flush(); err != nil {
		needImageRemove = true
		log.Printf("ceph: image flush error pool=%s name=%s: %s", pool, name, err)
		return err
	}

	// Create and protect snapshot
	snap, err := img.CreateSnapshot("base")
	if err != nil {
		needImageRemove = true
		log.Printf("ceph: image snapshot crate error pool=%s name=%s: %s", pool, name, err)
		return err
	}
	if err := snap.Protect(); err != nil {
		if snapErr := snap.Remove(); snapErr != nil {
			log.Printf("ceph: error removing snapshot: %s", snapErr)
		} else {
			needImageRemove = true
		}
		return err
	}
	return nil
}

func DeleteImage(pool, name string) error {
	// Create connection; defer shutdown
	conn, err := NewConnection()
	if err != nil {
		log.Printf("ceph: new connection failed: %s", err)
		return err
	}
	defer conn.Close()

	img := rbd.GetImage(conn.ioctx[pool], name)
	snapNames, err := img.GetSnapshotNames()
	if err != nil {
		return err
	}

	// Remove all snapshots, probably unprotecting them
	for _, snapName := range snapNames {
		snap := img.GetSnapshot(snapName.Name)
		isProtected, err := snap.IsProtected()
		if err != nil {
			log.Printf("ceph: failed to get  snapshot protected status name=%s image=%s", snapName, name)
			return err
		}
		if isProtected {
			if err := snap.Unprotect(); err != nil {
				log.Printf("ceph: failed to unprotect snapshot name=%s image=%s", snapName, name)
				return err
			}
		}
		if err := snap.Remove(); err != nil {
			log.Printf("ceph: failed to remove snapshot name=%s image=%s", snapName, name)
			return err
		}
	}

	// Remove the image
	if err := img.Remove(); err != nil {
		log.Printf("ceph: failed to remove image name=%s", name)
		return err
	}
	return nil
}
