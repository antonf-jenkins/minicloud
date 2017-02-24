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
	"log"
	"net/http"
	"strings"
)

type Handler func(context.Context, http.ResponseWriter, *http.Request, PathParams)

type link struct {
	prev *link
	len  int
	kv   KeyValue
}

type packedParams struct {
	ctx  context.Context
	req  *http.Request
	resp http.ResponseWriter
}

func processRest(pp *packedParams, node *MountPoint, rest string, chain *link) bool {
	if rest == "" {
		handler := node.handlers[pp.req.Method]
		if handler != nil {
			var params PathParams
			if chain != nil {
				params = make(PathParams, 0, chain.len)
				for it := chain; it != nil; it = it.prev {
					params = append(params, it.kv)
				}
			}
			handler(pp.ctx, pp.resp, pp.req, params)
		} else {
			handlersLen := len(node.handlers)
			if handlersLen == 0 {
				return false
			}
			methods := make([]string, 0, handlersLen)
			for key := range node.handlers {
				methods = append(methods, key)
			}
			Respond405(pp.resp, methods)
		}
		return true
	}
	for _, child := range node.children {
		proc := child.proc
		switch p := proc.(type) {
		case *namespaceProc:
			if p.process(pp, child, rest, chain) {
				return true
			}
		case *stringParamProc:
			if p.process(pp, child, rest, chain) {
				return true
			}
		case *ulidParamProc:
			if p.process(pp, child, rest, chain) {
				return true
			}
		}
	}
	return false
}

type MountPoint struct {
	handlers map[string]Handler
	children []*MountPoint
	source   string
	proc     interface{}
}

func (mp *MountPoint) Mount(method string, handler Handler) {
	if mp.handlers[method] != nil {
		log.Fatalf("api: mux: method '%s' already mounted at '%s'", method, mp.source)
		return
	}
	mp.handlers[method] = handler
}

func newMountPoint(source string) *MountPoint {
	return &MountPoint{
		handlers: make(map[string]Handler),
		source:   source,
		proc:     newProc(source),
	}
}

type namespaceProc struct {
	name string
}

func (np *namespaceProc) process(pp *packedParams, node *MountPoint, path string, chain *link) bool {
	elem, rest := nextPathElem(path)
	if elem == np.name {
		return processRest(pp, node, rest, chain)
	}
	return false
}

type stringParamProc struct {
	paramName string
}

func (spp *stringParamProc) process(pp *packedParams, node *MountPoint, path string, chain *link) bool {
	value, rest := nextPathElem(path)
	return processRest(pp, node, rest, &link{
		prev: chain,
		len:  nextLen(chain),
		kv:   KeyValue{Key: spp.paramName, StringValue: value},
	})
}

type ulidParamProc struct {
	paramName string
}

func (upp *ulidParamProc) process(pp *packedParams, node *MountPoint, path string, chain *link) bool {
	elem, rest := nextPathElem(path)
	id, err := ulid.Parse(elem)
	if err != nil {
		return false
	}
	return processRest(pp, node, rest, &link{
		prev: chain,
		len:  nextLen(chain),
		kv:   KeyValue{Key: upp.paramName, UlidValue: id},
	})
}

func newProc(source string) interface{} {
	elemLen := len(source)
	if elemLen > 2 && source[0] == '{' && source[elemLen-1] == '}' {
		nameType := strings.Split(source[1:elemLen-1], ":")
		name, ty := nameType[0], nameType[1]
		switch ty {
		case "string":
			return &stringParamProc{name}
		case "ulid":
			return &ulidParamProc{name}
		default:
			log.Panicf("api: mux: unknown parameter type: %s", ty)
			return nil
		}
	} else {
		if elemLen == 0 {
			return nil
		} else {
			return &namespaceProc{source}
		}
	}
}
