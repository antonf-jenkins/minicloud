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

type IntOpt interface {
	option
	Value() int
	Listen(callback func(newVal int)) CancelFunc
}

type implIntOpt struct {
	header
	CurrentValue, DefaultValue int
}

func (o *implIntOpt) Value() int {
	o.Lock()
	defer o.Unlock()
	o.checkInitialized()
	return o.CurrentValue
}

func (o *implIntOpt) Listen(callback func(newVal int)) CancelFunc {
	return o.listenImpl(func(newVal interface{}) {
		callback(newVal.(int))
	})
}

func NewIntOpt(name string, defaultValue int) IntOpt {
	option := &implIntOpt{
		header:       newHeader(name),
		CurrentValue: defaultValue,
		DefaultValue: defaultValue,
	}
	registerOpt(option)
	return option
}
