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
	"encoding/json"
	"github.com/antonf/minicloud/fsm"
	"github.com/oklog/ulid"
	"net/http"
	"reflect"
)

type managerHandlers struct {
	newRv    reflect.Value
	listRv   reflect.Value
	getRv    reflect.Value
	postRv   reflect.Value
	putRv    reflect.Value
	deleteRv reflect.Value
}

func (mh *managerHandlers) newEntity() reflect.Value {
	return mh.newRv.Call(nil)[0]
}

func (mh *managerHandlers) list(ctx context.Context) (reflect.Value, error) {
	result := mh.listRv.Call([]reflect.Value{reflect.ValueOf(ctx)})
	return result[0], toError(result[1])
}

func (mh *managerHandlers) get(ctx context.Context, id ulid.ULID) (reflect.Value, error) {
	result := mh.getRv.Call([]reflect.Value{reflect.ValueOf(ctx), reflect.ValueOf(id)})
	return result[0], toError(result[1])
}

func (mh *managerHandlers) create(ctx context.Context, entity reflect.Value) error {
	result := mh.postRv.Call([]reflect.Value{reflect.ValueOf(ctx), entity})
	return toError(result[0])
}

func (mh *managerHandlers) update(ctx context.Context, entity reflect.Value) error {
	result := mh.putRv.Call([]reflect.Value{reflect.ValueOf(ctx), entity, reflect.ValueOf(fsm.User)})
	return toError(result[0])
}

func (mh *managerHandlers) delete(ctx context.Context, id ulid.ULID) error {
	result := mh.deleteRv.Call([]reflect.Value{reflect.ValueOf(ctx), reflect.ValueOf(id)})
	return toError(result[0])
}

func adaptManager(manager interface{}) *managerHandlers {
	managerRv := reflect.ValueOf(manager)

	return &managerHandlers{
		newRv:    managerRv.MethodByName("NewEntity"),
		listRv:   managerRv.MethodByName("List"),
		getRv:    managerRv.MethodByName("Get"),
		postRv:   managerRv.MethodByName("Create"),
		putRv:    managerRv.MethodByName("Update"),
		deleteRv: managerRv.MethodByName("Delete"),
	}
}

func (mh *managerHandlers) handleList(ctx context.Context, w http.ResponseWriter, req *http.Request, params PathParams) {
	entities, err := mh.list(ctx)
	if err != nil {
		writeError(w, err)
		return
	}
	data, err := json.Marshal(entities.Interface())
	if err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusOK)
	w.Write(data)
}

func (mh *managerHandlers) handlePost(ctx context.Context, w http.ResponseWriter, req *http.Request, params PathParams) {
	entity := mh.newEntity()
	decoder := json.NewDecoder(req.Body)
	if err := decoder.Decode(entity.Interface()); err != nil {
		writeError(w, err)
		return
	}
	if err := mh.create(ctx, entity); err != nil {
		writeError(w, err)
		return
	}
	w.Header().Add(HeaderEntityId, getEntityId(entity).String())
	w.WriteHeader(http.StatusNoContent)
}

func (mh *managerHandlers) handleGet(ctx context.Context, w http.ResponseWriter, req *http.Request, params PathParams) {
	entity, err := mh.get(ctx, params.GetULID("id"))
	if err != nil {
		writeError(w, err)
		return
	}
	data, err := json.Marshal(entity.Interface())
	if err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusOK)
	w.Write(data)
}

func (mh *managerHandlers) handlePut(ctx context.Context, w http.ResponseWriter, req *http.Request, params PathParams) {
	entity, err := mh.get(ctx, params.GetULID("id"))
	if err != nil {
		writeError(w, err)
		return
	}
	decoder := json.NewDecoder(req.Body)
	if err := decoder.Decode(entity.Interface()); err != nil {
		writeError(w, err)
		return
	}
	if err := mh.update(ctx, entity); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (mh *managerHandlers) handleDelete(ctx context.Context, w http.ResponseWriter, req *http.Request, params PathParams) {
	err := mh.delete(ctx, params.GetULID("id"))
	if err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
