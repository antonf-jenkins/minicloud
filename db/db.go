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
	"github.com/oklog/ulid"
)

type Initiator int

const (
	InitiatorSystem Initiator = 1 << 0
	InitiatorUser   Initiator = 1 << 1
	MetaPrefix                = "/minicloud/db/meta"
)

type RawValue struct {
	CreateRev, ModifyRev int64
	Key                  string
	Data                 []byte
}

type Connection interface {
	RawRead(ctx context.Context, key string) (*RawValue, error)
	RawReadPrefix(ctx context.Context, key string) ([]RawValue, error)
	RawWatchPrefix(ctx context.Context, prefix string) chan *RawValue
	NewTransaction() Transaction
}

type Transaction interface {
	Commit(ctx context.Context) error
	Create(ctx context.Context, entity Entity)
	Update(ctx context.Context, entity Entity)
	Delete(ctx context.Context, entity Entity)
	CreateMeta(ctx context.Context, key, content string)
	CheckMeta(ctx context.Context, key, content string)
	DeleteMeta(ctx context.Context, key string)
	AcquireLock(ctx context.Context, key string)
	ReleaseLock(ctx context.Context, key string)
}

type Entity interface {
	Header() *EntityHeader
	EntityName() string
}

type EntityHeader struct {
	SchemaVersion int64
	CreateRev     int64  `json:"-"`
	ModifyRev     int64  `json:"-"`
	Original      Entity `json:"-"`
	Id            ulid.ULID
	State         State
}

func (hdr *EntityHeader) Header() *EntityHeader {
	return hdr
}
