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
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"github.com/antonf/minicloud/utils"
	"sync/atomic"
	"syscall"
	"unsafe"
)

var (
	nextSeqNr   uint32
	nativeOrder binary.ByteOrder
)

func init() {
	var x uint32 = 0x01020304
	if *(*byte)(unsafe.Pointer(&x)) == 0x01 {
		nativeOrder = binary.BigEndian
	} else {
		nativeOrder = binary.LittleEndian
	}
}

type NetlinkSocket struct {
	fd int
	sa syscall.SockaddrNetlink
}

func NewNetlinkSocket() (*NetlinkSocket, error) {
	fd, err := syscall.Socket(syscall.AF_NETLINK, syscall.SOCK_RAW, syscall.NETLINK_ROUTE)
	if err != nil {
		return nil, err
	}
	nlsa := syscall.SockaddrNetlink{Family: syscall.AF_NETLINK}
	if err := syscall.Bind(fd, &nlsa); err != nil {
		return nil, err
	}
	return &NetlinkSocket{fd, nlsa}, nil
}

func (s *NetlinkSocket) Close() {
	syscall.Close(s.fd)
}

func (s *NetlinkSocket) Send(nlType, flags uint16, data interface{}) (uint32, error) {
	seq := atomic.AddUint32(&nextSeqNr, 1)
	buf := bytes.Buffer{}
	buf.Grow(256)
	binary.Write(&buf, nativeOrder, syscall.NlMsghdr{0, nlType, flags, seq, 0})
	binary.Write(&buf, nativeOrder, data)
	bytes := buf.Bytes()
	nativeOrder.PutUint32(bytes, uint32(buf.Len()))
	if err := syscall.Sendto(s.fd, bytes, 0, &s.sa); err != nil {
		return 0, err
	}
	return seq, nil
}

func (s *NetlinkSocket) Receive(ctx context.Context) ([]syscall.NetlinkMessage, error) {
	if value, err := utils.WrapContext(ctx, func() (interface{}, error) {
		buf := make([]byte, syscall.Getpagesize())
		nr, _, err := syscall.Recvfrom(s.fd, buf, 0)
		if err != nil {
			return nil, err
		}
		if nr < syscall.NLMSG_HDRLEN {
			return nil, syscall.EINVAL
		}
		return syscall.ParseNetlinkMessage(buf[:nr])
	}); err != nil {
		return nil, err
	} else {
		return value.([]syscall.NetlinkMessage), nil
	}
}

func (s *NetlinkSocket) WaitAck(ctx context.Context, seq uint32) error {
	pid := syscall.Getpid()
	for {
		msgs, err := s.Receive(ctx)
		if err != nil {
			return err
		}
		for _, m := range msgs {
			if seq != m.Header.Seq {
				return fmt.Errorf("Invalid seq: %d != %d", seq, m.Header.Seq)
			}
			if pid != int(m.Header.Pid) {
				return fmt.Errorf("Invalid pid: %d != %d", pid, m.Header.Pid)
			}
			if m.Header.Type == syscall.NLMSG_DONE {
				return nil
			}
			if m.Header.Type == syscall.NLMSG_ERROR {
				error := int32(nativeOrder.Uint32(m.Data[0:4]))
				if error == 0 {
					return nil
				}
				return syscall.Errno(-error)
			}
		}
	}
}
