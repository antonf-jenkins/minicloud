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
package qemu

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/antonf/minicloud/config"
	"github.com/antonf/minicloud/utils"
	"github.com/oklog/ulid"
	"io"
	"log"
	"net"
	"os"
	"sync"
	"syscall"
	"time"
)

var OptMonitorConnectTimeout = config.NewDurationOpt(
	"qemu_monitor_connect_timeout", 5*time.Second)

type Monitor struct {
	sync.Mutex
	path      string
	conn      net.Conn
	responses map[ulid.ULID]chan *response
	done      chan struct{}
}

type request struct {
	Id        ulid.ULID   `json:"id"`
	Execute   string      `json:"execute"`
	Arguments interface{} `json:"arguments,omitempty"`
}

type response struct {
	Id        ulid.ULID       `json:"id"`
	Error     *QmpError       `json:"error"`
	Return    json.RawMessage `json:"return"`
	Event     string          `json:"event"`
	Data      json.RawMessage `json:"data"`
	Timestamp struct {
		Seconds      int64 `json:"seconds"`
		Microseconds int64 `json:"microseconds"`
	} `json:"timestamp"`
}

type QmpError struct {
	Class string `json:"class"`
	Desc  string `json:"desc"`
}

func (e *QmpError) Error() string {
	return fmt.Sprintf("%s: %s", e.Class, e.Desc)
}

func NewMonitor(ctx context.Context, path string) (*Monitor, error) {
	d := net.Dialer{
		Timeout: OptMonitorConnectTimeout.Value(),
	}
	mon := &Monitor{
		path:      path,
		responses: make(map[ulid.ULID]chan *response),
		done:      make(chan struct{}),
	}
	backoff := utils.NewBackoff(100*time.Millisecond, OptMonitorConnectTimeout.Value())
	for {
		if conn, err := d.DialContext(ctx, "unix", path); err != nil {
			if opError, ok := err.(*net.OpError); ok {
				if syscallError, ok := opError.Err.(*os.SyscallError); ok {
					switch syscallError.Err {
					case syscall.ENOENT:
					case syscall.ECONNREFUSED:
					case syscall.EAGAIN:
					case syscall.EINPROGRESS:
					default:
						return nil, err
					}
					if backoff.Wait() {
						log.Printf("qemu: connection to %s failed: %s; retrying", path, syscallError)
						continue
					}
				}
			}
			return nil, err
		} else {
			mon.conn = conn
			break
		}
	}

	if err := mon.handshake(ctx); err != nil {
		log.Printf("qemu: handshake failed: %s", err)
		mon.conn.Close()
		return nil, err
	}

	go mon.decodeResponses(ctx)
	if err := mon.qmpCapabilities(ctx); err != nil {
		log.Printf("qemu: qmp_capabilities failed: %s %T %s", err, err, err != nil)
		mon.Close()
		return nil, err
	}

	return mon, nil
}

func (mon *Monitor) Close() {
	mon.Lock()
	defer mon.Unlock()
	close(mon.done)
}

func (mon *Monitor) handshake(ctx context.Context) error {
	var hello struct {
		QMP struct {
			Version struct {
				Qemu struct {
					Major int `json:"major"`
					Minor int `json:"minor"`
					Micro int `json:"micro"`
				} `json:"qemu"`
				Package string `json:"package"`
			} `json:"version"`
			Capabilities []string `json:"capabilities"`
		}
	}

	ch := make(chan error)
	go func() {
		decoder := json.NewDecoder(mon.conn)
		ch <- decoder.Decode(&hello)
	}()

	select {
	case <-ctx.Done():
		return utils.ErrInterrupted
	case err := <-ch:
		if err != nil {
			return err
		}
		log.Printf("qemu: hello recieved: %+v", hello)
		return nil
	}
}

func (mon *Monitor) prepareRequest(command string, args interface{}) (*request, chan *response) {
	req := request{
		Id:        utils.NewULID(),
		Execute:   command,
		Arguments: args,
	}
	ch := make(chan *response, 1)
	mon.Lock()
	defer mon.Unlock()
	mon.responses[req.Id] = ch
	return &req, ch
}

func (mon *Monitor) closeRequest(id ulid.ULID) {
	mon.Lock()
	defer mon.Unlock()
	delete(mon.responses, id)
}

func (mon *Monitor) sendResponse(resp *response) {
	mon.Lock()
	defer mon.Unlock()
	ch := mon.responses[resp.Id]
	if ch != nil {
		ch <- resp
	} else {
		log.Printf("qemu: unexpected response %+v", resp)
	}
}

func (mon *Monitor) decodeResponses(ctx context.Context) {
	decoder := json.NewDecoder(mon.conn)
	ch := make(chan *response)
	for {
		// go receive one response
		go func() {
			resp := &response{}
			if err := decoder.Decode(resp); err != nil {
				if err == io.EOF {
					return
				}
				log.Printf("qemu: monitor %s error: %s", mon.path, err)
				ch <- nil
			} else {
				ch <- resp
			}
		}()
		select {
		case <-ctx.Done():
			mon.conn.Close()
			return
		case <-mon.done:
			mon.conn.Close()
			return
		case resp := <-ch:
			if resp != nil {
				if resp.Id.Time() != 0 {
					mon.sendResponse(resp)
				} else {
					timestamp := time.Unix(
						resp.Timestamp.Seconds,
						resp.Timestamp.Microseconds*1000)
					log.Printf("qemu: event=%s timestamp=%s data=%s", resp.Event, timestamp, string(resp.Data))
				}
			} else {
				return
			}
		}
	}
}

func (mon *Monitor) voidCommand(ctx context.Context, cmdName string, args interface{}) error {
	// Prepare request
	req, ch := mon.prepareRequest(cmdName, args)
	defer mon.closeRequest(req.Id)

	// Issue request
	enc := json.NewEncoder(mon.conn)
	if err := enc.Encode(req); err != nil {
		return err
	}

	// Wait for response
	select {
	case resp := <-ch:
		if resp.Error != nil {
			return resp.Error
		} else {
			return nil
		}
	case <-ctx.Done():
		return utils.ErrInterrupted
	}
}

func (mon *Monitor) qmpCapabilities(ctx context.Context) error {
	return mon.voidCommand(ctx, "qmp_capabilities", nil)
}

func (mon *Monitor) Cont(ctx context.Context) error {
	return mon.voidCommand(ctx, "cont", nil)
}

func (mon *Monitor) Stop(ctx context.Context) error {
	return mon.voidCommand(ctx, "stop", nil)
}

func (mon *Monitor) Quit(ctx context.Context) error {
	return mon.voidCommand(ctx, "quit", nil)
}
