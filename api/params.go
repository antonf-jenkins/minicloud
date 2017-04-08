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
	"github.com/oklog/ulid"
)

type KeyValue struct {
	Key         string
	StringValue string
	UlidValue   ulid.ULID
}

type PathParams []KeyValue

func (params PathParams) Keys() []string {
	keys := make([]string, len(params))
	for i, kv := range params {
		keys[i] = kv.Key
	}
	return keys
}

func (params PathParams) GetULID(ctx context.Context, name string) ulid.ULID {
	for i := range params {
		if params[i].Key == name {
			return params[i].UlidValue
		}
	}
	logger.Panic(ctx, "parameter not found", "name", name, "valid", params.Keys())
	return ulid.ULID{}
}

func (params PathParams) GetString(ctx context.Context, name string) string {
	for i := range params {
		if params[i].Key == name {
			return params[i].StringValue
		}
	}
	logger.Panic(ctx, "parameter not found", "name", name, "valid", params.Keys())
	return ""
}
