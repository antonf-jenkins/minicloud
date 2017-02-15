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
package db

import (
	"github.com/antonf/minicloud/env"
	backend "github.com/coreos/etcd/clientv3"
	"log"
	"strings"
	"time"
)

var globalEtcdConn *backend.Client

func init() {
	log.Printf("db: init: connecting to %s, timeout %dms...", env.EtcdEndpoints, env.EtcdDialTimeout)
	cli, err := backend.New(backend.Config{
		Endpoints:   strings.Split(env.EtcdEndpoints, ","),
		DialTimeout: time.Duration(env.EtcdDialTimeout) * time.Millisecond,
	})
	if err != nil {
		log.Fatalf("db: init: failed to connect to etcd cluster: %s", err)
	}

	log.Printf("db: init: connected to etcd cluster")
	globalEtcdConn = cli
}
