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
	"net"
	"os"
	"strings"
	"syscall"
	"unsafe"
)

func (vm *VirtualMachine) createTap(ctx context.Context, netdev NetworkDevice) (int, error) {
	file, fd, err := vm.openExtraFile("/dev/net/tun", os.O_RDWR)
	if err != nil {
		return 0, err
	}
	iface, err := allocTapIface(file.Fd(), netdev.InterfaceName)
	if err != nil {
		return 0, err
	}
	if err = bringIfaceUp(ctx, iface); err != nil {
		return 0, err
	}
	return fd, nil
}

func allocTapIface(fd uintptr, name string) (*net.Interface, error) {
	type ifreqFlags struct {
		Name  [0x10]byte
		Flags uint16
		pad   [0x28 - 0x10 - 0x02]byte
	}
	req := ifreqFlags{Flags: syscall.IFF_TAP | syscall.IFF_NO_PI}
	copy(req.Name[:], name)
	_, _, errno := syscall.Syscall(
		syscall.SYS_IOCTL,
		fd,
		uintptr(syscall.TUNSETIFF),
		uintptr(unsafe.Pointer(&req)))
	if errno != 0 {
		return nil, errno
	}
	return net.InterfaceByName(strings.Trim(string(req.Name[:]), "\x00"))
}

func bringIfaceUp(ctx context.Context, iface *net.Interface) error {
	s, err := NewNetlinkSocket()
	if err != nil {
		return err
	}
	defer s.Close()

	msg := syscall.IfInfomsg{
		Family: syscall.AF_UNSPEC,
		Index:  int32(iface.Index),
		Flags:  syscall.IFF_UP,
		Change: syscall.IFF_UP,
	}
	seq, err := s.Send(syscall.RTM_NEWLINK, syscall.NLM_F_REQUEST|syscall.NLM_F_ACK, msg)
	if err != nil {
		return err
	}

	return s.WaitAck(ctx, seq)
}
