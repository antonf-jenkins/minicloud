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
	"os"
	"reflect"
	"strings"
	"sync"
)

const GlobalConfigPrefix = "/minicloud/config/global"

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
		fmt.Fprintf(os.Stderr, "Option '%s' value used before init", hdr.name)
		os.Exit(1)
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
		fmt.Fprintf(os.Stderr, "Duplicate option name: %s", hdr.name)
		os.Exit(1)
	}
	options[hdr.name] = opt
}

func processConfigEvents(ctx context.Context, rawValueCh chan *db.RawValue) {
	for {
		select {
		case <-ctx.Done():
			return
		case rawValue := <-rawValueCh:
			if opt := getOption(ctx, rawValue); opt != nil {
				updateOption(ctx, opt, rawValue)
			}
		}
	}
}

func initializeOpt(ctx context.Context, opt option, conn db.Connection) {
	hdr := opt.headerPtr()
	key := fmt.Sprintf("%s/%s", GlobalConfigPrefix, hdr.name)
	if rawValue, err := conn.RawRead(ctx, key); err != nil {
		fmt.Printf("config: error getting opt '%s' initial value: %s", hdr.name, err)
	} else {
		updateOption(ctx, opt, rawValue)
	}
}

func updateOption(ctx context.Context, opt option, rawValue *db.RawValue) {
	hdr := opt.headerPtr()
	hdr.Lock()
	defer hdr.Unlock()
	if rawValue.ModifyRev < hdr.rev {
		logger.Notice(ctx, "ignoring stale value", "value_rev", rawValue.ModifyRev, "option_rev", hdr.rev)
		return
	}
	hdr.rev = rawValue.ModifyRev
	optRv := reflect.ValueOf(opt).Elem()
	curValRv := optRv.FieldByName("CurrentValue")
	if rawValue.Data != nil {
		newValRv := reflect.New(curValRv.Type()).Elem()
		if err := json.Unmarshal(rawValue.Data, newValRv.Addr().Interface()); err != nil {
			logger.Error(ctx, "unmarshal failed", "option", hdr.name, "data", string(rawValue.Data))
		} else {
			if hdr.update(curValRv, newValRv) {
				logger.Info(ctx, "option updated", "option", hdr.name, "rev", hdr.rev, "value", newValRv)
			}
			return
		}
	}
	defaultValRv := optRv.FieldByName("DefaultValue")
	if hdr.update(curValRv, defaultValRv) {
		logger.Info(ctx, "option reset to default", "option", hdr.name, "rev", hdr.rev, "value", defaultValRv)
	}
}

func getOption(ctx context.Context, rawValue *db.RawValue) option {
	optionsMutex.Lock()
	defer optionsMutex.Unlock()
	optionName := rawValue.Key[strings.LastIndex(rawValue.Key, "/")+1:]
	option, optionExists := options[optionName]
	if !optionExists {
		logger.Warn(ctx, "unknown option", "option", optionName, "key", rawValue.Key)
		return nil
	}
	return option
}

func init() {
	options = make(map[string]option)
	initCommonOptions()
}

func InitOptions(ctx context.Context, conn db.Connection) {
	rawValCh := conn.RawWatchPrefix(ctx, GlobalConfigPrefix)
	for _, opt := range options {
		initializeOpt(ctx, opt, conn)
	}
	go processConfigEvents(ctx, rawValCh)
}
