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
package config

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/antonf/minicloud/db"
	"log"
	"reflect"
	"strings"
	"sync"
)

const GlobalPrefix = "/minicloud/config"

type CancelFunc func()

type option interface {
	headerPtr() *header
}

type header struct {
	sync.Mutex
	name       string
	rev        int64
	nextHandle int
	listeners  map[int]func(interface{})
}

var (
	optionsMutex sync.Mutex
	options      map[string]option
)

func (hdr *header) listenImpl(callback func(newVal interface{})) CancelFunc {
	hdr.Lock()
	defer hdr.Unlock()
	handle := hdr.nextHandle
	hdr.nextHandle += 1
	hdr.listeners[handle] = callback
	return func() {
		hdr.Lock()
		defer hdr.Unlock()
		delete(hdr.listeners, handle)
	}
}

func (hdr *header) headerPtr() *header {
	return hdr
}

func (hdr *header) update(curValRv, newValRv reflect.Value) bool {
	if reflect.DeepEqual(curValRv.Interface(), newValRv.Interface()) {
		return false
	}
	curValRv.Set(newValRv)
	curVal := curValRv.Interface()
	for _, listener := range hdr.listeners {
		listener(curVal)
	}
	return true
}

func (hdr *header) checkInitialized() {
	if hdr.rev < 0 {
		log.Fatalf("config: opt: value used before init")
	}
}

func newHeader(name string) header {
	return header{
		name:      name,
		rev:       -1,
		listeners: make(map[int]func(interface{})),
	}
}

func registerOpt(opt option) {
	optionsMutex.Lock()
	defer optionsMutex.Unlock()
	hdr := opt.headerPtr()
	if _, ok := options[hdr.name]; ok {
		log.Fatalf("config: opt '%s' already exists", hdr.name)
	}
	options[hdr.name] = opt
}

func processConfigEvents(ctx context.Context, rawValueCh chan *db.RawValue) {
	for {
		select {
		case <-ctx.Done():
			return
		case rawValue := <-rawValueCh:
			if opt := getOption(rawValue); opt != nil {
				updateOption(opt, rawValue)
			}
		}
	}
}

func initializeOpt(ctx context.Context, opt option, conn db.Connection) {
	hdr := opt.headerPtr()
	key := fmt.Sprintf("%s/%s", GlobalPrefix, hdr.name)
	if rawValue, err := conn.RawRead(ctx, key); err != nil {
		fmt.Printf("config: error getting opt '%s' initial value: %s", hdr.name, err)
	} else {
		updateOption(opt, rawValue)
	}
}


func updateOption(opt option, rawValue *db.RawValue) {
	hdr := opt.headerPtr()
	hdr.Lock()
	defer hdr.Unlock()
	if rawValue.ModifyRev < hdr.rev {
		return
	}
	hdr.rev = rawValue.ModifyRev
	optRv := reflect.ValueOf(opt).Elem()
	curValRv := optRv.FieldByName("CurrentValue")
	if rawValue.Data != nil {
		newValRv := reflect.New(curValRv.Type()).Elem()
		if err := json.Unmarshal(rawValue.Data, newValRv.Addr().Interface()); err != nil {
			log.Printf("config: opt: unmarshal '%s' failed: %s", hdr.name, err)
		} else {
			if hdr.update(curValRv, newValRv) {
				log.Printf("config: opt: update '%s': [%d] %s", hdr.name, hdr.rev, newValRv)
			}
			return
		}
	}
	defaultValRv := optRv.FieldByName("DefaultValue")
	if hdr.update(curValRv, defaultValRv) {
		log.Printf("config: opt: reset to default '%s': [%d] %s", hdr.name, hdr.rev, defaultValRv)
	}
}

func getOption(rawValue *db.RawValue) option {
	optionsMutex.Lock()
	defer optionsMutex.Unlock()
	optionName := rawValue.Key[strings.LastIndex(rawValue.Key, "/")+1:]
	option, optionExists := options[optionName]
	if !optionExists {
		log.Printf("config: unknown opt: %s (key=%s)", optionName, rawValue.Key)
		return nil
	}
	return option
}

func init() {
	options = make(map[string]option)
}

func InitOptions(ctx context.Context, conn db.Connection) {
	rawValCh := make(chan *db.RawValue)
	conn.RawWatchPrefix(ctx, GlobalPrefix, rawValCh)
	for _, opt := range options {
		initializeOpt(ctx, opt, conn)
	}
	go processConfigEvents(ctx, rawValCh)
}
