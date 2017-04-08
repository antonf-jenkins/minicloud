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
	"fmt"
	"github.com/antonf/minicloud/utils"
	"os"
	"os/exec"
	"path"
)

const (
	baseCmd = "qemu-system-x86_64"
)

var baseOptions = []string{
	"-S",
	"-no-user-config",
	"-nodefconfig",
	"-nodefaults",
	"-global", "kvm-pit.lost_tick_policy=discard",
	"-global", "PIIX4_PM.disable_s3=1",
	"-global", "PIIX4_PM.disable_s4=1",
	"-machine", "pc-i440fx-2.8,accel=kvm,usb=off,vmport=off,mem-merge=off",
	"-rtc", "base=utc,clock=host,driftfix=none",
	"-no-hpet",
	"-no-shutdown",
	"-boot", "strict=on",
	"-vga", "std",
}

func (vm *VirtualMachine) appendArgs(args ...string) {
	vm.cmd.Args = append(vm.cmd.Args, args...)
}

func (vm *VirtualMachine) appendMonitor() {
	monPath := path.Join(vm.Root, "mon.sock")
	socketCharDev := fmt.Sprintf("socket,id=charmon,path=%s,server,nowait", monPath)
	vm.appendArgs("-chardev", socketCharDev)
	vm.appendArgs("-mon", "chardev=charmon,mode=control")
}

func (vm *VirtualMachine) appendVnc() {
	// TODO: SSL protected VNC
	vm.appendArgs("-vnc", fmt.Sprintf("0.0.0.0:%d", vm.VncPort))
}

func (vm *VirtualMachine) openFile(path string, flag int, perm os.FileMode) (*os.File, error) {
	file, err := os.OpenFile(path, flag, perm)
	if err != nil {
		return nil, err
	}
	vm.files = append(vm.files, file)
	return file, nil
}

func (vm *VirtualMachine) openLocalFile(name string, flag int, perm os.FileMode) (*os.File, error) {
	return vm.openFile(path.Join(vm.Root, name), flag, perm)
}

func (vm *VirtualMachine) openExtraFile(path string, flag int) (*os.File, int, error) {
	// ExtraFiles[i] will become fd=i+3 in child process
	fd := len(vm.cmd.ExtraFiles) + 3
	file, err := vm.openFile(path, flag, 0)
	if err != nil {
		return nil, 0, err
	}
	vm.cmd.ExtraFiles = append(vm.cmd.ExtraFiles, file)
	return file, fd, nil
}

func (vm *VirtualMachine) closeFiles() {
	for _, file := range vm.files {
		file.Close()
	}
	vm.files = nil
}

func (vm *VirtualMachine) prepareCommand(ctx context.Context) (err error) {
	if vm.cmd != nil {
		return fmt.Errorf("Tried to start vm %s twice", vm.Id)
	}
	vm.cmd = exec.CommandContext(ctx, baseCmd, baseOptions...)
	logFlags := os.O_WRONLY | os.O_APPEND | os.O_CREATE
	if vm.cmd.Stdout, err = vm.openLocalFile("stdout.log", logFlags, 0600); err != nil {
		return
	}
	if vm.cmd.Stderr, err = vm.openLocalFile("stderr.log", logFlags, 0600); err != nil {
		return
	}

	vm.appendMonitor()
	vm.appendArgs("-uuid", utils.ConvertToUUID(vm.Id))
	vm.appendArgs("-cpu", vm.Cpu)
	if vm.MemLock {
		vm.appendArgs("-realtime", "mlock=on")
	} else {
		vm.appendArgs("-realtime", "mlock=off")
	}

	for _, disk := range vm.Disks {
		vm.appendArgs("-drive", fmt.Sprintf(
			"format=rbd,file=rbd:%s/%s,if=virtio,discard=on,cache=%s",
			disk.Pool, disk.Disk, disk.Cache))
	}

	vhost := "off"
	if vm.VhostNet {
		vhost = "on"
	}
	for idx, netdev := range vm.Nics {
		var tapFd int
		tapFd, err = vm.createTap(ctx, netdev)
		if err != nil {
			return
		}
		netdevOpt := fmt.Sprintf("tap,id=nic%d,vhost=%s,fd=%d", idx, vhost, tapFd)
		if vm.VhostNet {
			var vhostFd int
			if _, vhostFd, err = vm.openExtraFile("/dev/vhost-net", os.O_RDWR); err == nil {
				netdevOpt += fmt.Sprintf(",vhostfd=%d", vhostFd)
			} else {
				return
			}
		}
		vm.appendArgs("-netdev", netdevOpt)
		vm.appendArgs("-device", fmt.Sprintf(
			"virtio-net-pci,netdev=nic%d,id=virtionet%d,mac=%s",
			idx, idx, netdev.MacAddress))
	}

	vm.appendVnc()

	return nil
}

func (vm *VirtualMachine) Start(ctx context.Context) error {
	defer vm.closeFiles()
	if err := vm.prepareCommand(ctx); err != nil {
		return err
	}
	if err := vm.cmd.Start(); err != nil {
		return err
	}
	vm.closeFiles()
	if mon, err := NewMonitor(ctx, path.Join(vm.Root, "mon.sock")); err != nil {
		if killErr := vm.cmd.Process.Kill(); killErr != nil {
			logger.Error(ctx, "failed to kill process", "pid", vm.cmd.Process.Pid, "error", killErr)
		}
		return err
	} else {
		vm.mon = mon
	}

	return nil
}

func (vm *VirtualMachine) Wait() error {
	return vm.cmd.Wait()
}
