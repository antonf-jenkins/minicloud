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

type StringOpt interface {
	option
	Value() string
	Listen(callback func (newVal string)) CancelFunc
}

type implStringOpt struct {
	header
	CurrentValue, DefaultValue string
}

func (o *implStringOpt) Value() string {
	o.Lock()
	defer o.Unlock()
	o.checkInitialized()
	return o.CurrentValue
}

func (o *implStringOpt) Listen(callback func (newVal string)) CancelFunc {
	return o.listenImpl(func(newVal interface{}) {
		callback(newVal.(string))
	})
}

func NewStringOpt(name string, defaultValue string) StringOpt {
	option := &implStringOpt{
		header: newHeader(name),
		CurrentValue: defaultValue,
		DefaultValue: defaultValue,
	}
	registerOpt(option)
	return option
}
