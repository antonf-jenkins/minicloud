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
package dbimpl

import (
	"context"
	"github.com/antonf/minicloud/db"
	"github.com/antonf/minicloud/env"
	"github.com/antonf/minicloud/log"
	backend "github.com/coreos/etcd/clientv3"
	"strings"
	"time"
)

type etcdConeection struct {
	client         *backend.Client
	projectManager *etcdProjectManager
	imageManager   *etcdImageManager
	diskManager    *etcdDiskManager
}

func (c *etcdConeection) Projects() db.ProjectManager {
	return c.projectManager
}

func (c *etcdConeection) Images() db.ImageManager {
	return c.imageManager
}

func (c *etcdConeection) Disks() db.DiskManager {
	return c.diskManager
}

func (c *etcdConeection) RawRead(ctx context.Context, key string) (*db.RawValue, error) {
	resp, err := c.client.Get(ctx, key, backend.WithSerializable())
	if err != nil {
		return nil, err
	}
	result := &db.RawValue{}
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

func (c *etcdConeection) RawReadPrefix(ctx context.Context, key string) ([]db.RawValue, error) {
	resp, err := c.client.Get(ctx, key, backend.WithSerializable(), backend.WithPrefix())
	if err != nil {
		return nil, err
	}
	result := make([]db.RawValue, resp.Count)
	for i, kv := range resp.Kvs {
		result[i].CreateRev = kv.CreateRevision
		result[i].ModifyRev = kv.ModRevision
		result[i].Key = string(kv.Key)
		result[i].Data = kv.Value
	}
	return result, nil
}

func (c *etcdConeection) RawWatchPrefix(ctx context.Context, prefix string) chan *db.RawValue {
	respCh := c.client.Watch(ctx, prefix, backend.WithPrefix())
	resultCh := make(chan *db.RawValue)
	go func() {
		logger.Debug(ctx, "watching prefix", "prefix", prefix)
		for {
			select {
			case <-ctx.Done():
				logger.Debug(ctx, "stopped watching prefix", "prefix", prefix)
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
					resultCh <- &db.RawValue{kv.CreateRevision, kv.ModRevision, key, value}
				}
			}
		}
	}()
	return resultCh
}

func NewConnection(ctx context.Context) db.Connection {
	opCtx := log.WithValues(ctx, "endpoints", env.EtcdEndpoints, "timeout", env.EtcdDialTimeout)
	logger.Debug(opCtx, "connecting to etcd")
	cli, err := backend.New(backend.Config{
		Endpoints:   strings.Split(env.EtcdEndpoints, ","),
		DialTimeout: time.Duration(env.EtcdDialTimeout) * time.Millisecond,
	})
	if err != nil {
		logger.Error(opCtx, "error connecting to etcd", "error", err)
	}

	logger.Info(opCtx, "connected to etcd cluster")
	conn := &etcdConeection{client: cli}
	conn.projectManager = &etcdProjectManager{conn}
	conn.imageManager = &etcdImageManager{conn}
	conn.diskManager = &etcdDiskManager{conn}
	return conn
}
