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
	"context"
	"github.com/antonf/minicloud/env"
	backend "github.com/coreos/etcd/clientv3"
	"log"
	"strings"
	"time"
)

type RawValue struct {
	CreateRev, ModifyRev int64
	Key                  string
	Data                 []byte
}

type Connection interface {
	RawRead(ctx context.Context, key string) (*RawValue, error)
	RawWatchPrefix(ctx context.Context, prefix string) chan *RawValue

	Projects() ProjectManager
	Images() ImageManager
	Disks() DiskManager
}

type etcdConeection struct {
	client         *backend.Client
	projectManager *etcdProjectManager
	imageManager   *etcdImageManager
	diskManager    *etcdDiskManager
}

func (c *etcdConeection) Projects() ProjectManager {
	return c.projectManager
}

func (c *etcdConeection) Images() ImageManager {
	return c.imageManager
}

func (c *etcdConeection) Disks() DiskManager {
	return c.diskManager
}

func (db *etcdConeection) RawRead(ctx context.Context, key string) (*RawValue, error) {
	resp, err := db.client.Get(ctx, key, backend.WithSerializable())
	if err != nil {
		return nil, err
	}
	result := &RawValue{}
	if resp.Count == 0 {
		result.ModifyRev = resp.Header.Revision
	} else {
		kv := resp.Kvs[0]
		result.CreateRev = kv.CreateRevision
		result.ModifyRev = kv.ModRevision
		result.Data = kv.Value
	}
	return result, nil
}

func (db *etcdConeection) RawWatchPrefix(ctx context.Context, prefix string) chan *RawValue {
	respCh := db.client.Watch(ctx, prefix, backend.WithPrefix())
	resultCh := make(chan *RawValue)
	go func() {
		log.Printf("db: raw: watching prefix %s", prefix)
		for {
			select {
			case <-ctx.Done():
				log.Printf("db: raw: stopped watching prefix %s", prefix)
				close(resultCh)
				return
			case eventBatch := <-respCh:
				for _, ev := range eventBatch.Events {
					var value []byte
					kv := ev.Kv
					key := string(kv.Key)
					if ev.Type == backend.EventTypePut {
						value = kv.Value
					}
					resultCh <- &RawValue{kv.CreateRevision, kv.ModRevision, key, value}
				}
			}
		}
	}()
	return resultCh
}

func NewConnection() Connection {
	log.Printf("db: init: connecting to %s, timeout %dms...", env.EtcdEndpoints, env.EtcdDialTimeout)
	cli, err := backend.New(backend.Config{
		Endpoints:   strings.Split(env.EtcdEndpoints, ","),
		DialTimeout: time.Duration(env.EtcdDialTimeout) * time.Millisecond,
	})
	if err != nil {
		log.Fatalf("db: init: failed to connect to etcd cluster: %s", err)
	}

	log.Printf("db: init: connected to etcd cluster")
	conn := &etcdConeection{client: cli}
	conn.projectManager = &etcdProjectManager{conn}
	conn.imageManager = &etcdImageManager{conn}
	return conn
}
