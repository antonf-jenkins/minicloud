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
package utils

import "context"

type valerr struct {
	panic interface{}
	val   interface{}
	err   error
}

func unblockOnPanic(ch chan *valerr) {
	if r := recover(); r != nil {
		ch <- &valerr{
			panic: r,
		}
	}
}

func WrapContext(ctx context.Context, operation func() (interface{}, error)) (interface{}, error) {
	ch := make(chan *valerr)
	go func() {
		defer unblockOnPanic(ch)
		var r valerr
		r.val, r.err = operation()
		ch <- &r
	}()

	select {
	case <-ctx.Done():
		return nil, ErrInterrupted
	case r := <-ch:
		if r.panic != nil {
			panic(r)
		}
		return r.val, r.err
	}
}
