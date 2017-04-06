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
	"github.com/antonf/minicloud/config"
	"github.com/ceph/go-ceph/rados"
	"log"
)

var (
	OptMonHost    = config.NewStringOpt("ceph_mon_host", "127.0.0.1")
	OptKey        = config.NewStringOpt("ceph_key", "")
	OptImageOrder = config.NewIntOpt("ceph_image_order", 18)
	OptDiskOrder  = config.NewIntOpt("ceph_disk_order", 18)
)

type connection struct {
	conn  *rados.Conn
	ioctx map[string]*rados.IOContext
}

func setConfigOptions(conn *rados.Conn, options ...string) error {
	optLen := len(options)
	if optLen%2 != 0 {
		panic("Odd number of arguments passed to setConfigOptions")
	}
	for i := 0; i < optLen; i += 2 {
		key := options[i]
		value := options[i+1]
		if err := conn.SetConfigOption(key, value); err != nil {
			log.Printf("ceph: rados: error setting option %s=%s: %s", key, value, err)
			return err
		}
	}
	return nil
}

func NewConnection(pools ...string) (*connection, error) {
	conn, err := rados.NewConn()
	if err != nil {
		log.Printf("ceph: new conn error: %s", err)
		return nil, err
	}
	err = setConfigOptions(conn,
		"mon_host", OptMonHost.Value(),
		"key", OptKey.Value())
	if err != nil {
		return nil, err
	}
	if err := conn.Connect(); err != nil {
		return nil, err
	}

	result := connection{conn, make(map[string]*rados.IOContext)}
	for _, pool := range pools {
		if ioctx, err := conn.OpenIOContext(pool); err != nil {
			log.Printf("ceph: open ioctx pool=%s: %s", pool, err)
			result.Close()
			return nil, err
		} else {
			result.ioctx[pool] = ioctx
		}
	}

	return &result, nil
}

func (c *connection) Close() {
	if c.ioctx != nil {
		for _, ioctx := range c.ioctx {
			ioctx.Destroy()
		}
		c.ioctx = nil
	}
	if c.conn != nil {
		c.conn.Shutdown()
		c.conn = nil
	}
}
