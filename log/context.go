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
package log

import (
	"context"
)

type key int

var (
	logContext key = 0
)

type NamedValue struct {
	next  *NamedValue
	Name  string
	Value interface{}
}

func mergeStructuredData(ctx context.Context, args []interface{}) *NamedValue {
	var oldLogContext, list, prev *NamedValue

	if ctx != nil {
		oldLogContext, _ = ctx.Value(logContext).(*NamedValue)
	}

	// Build named value list
	for i := 0; i < len(args); {
		nv := &NamedValue{next: list}
		list = nv
		havePair := false
		if name, ok := args[i].(string); ok && i+1 < len(args) {
			nv.Name = name
			nv.Value = args[i+1]
			havePair = true
			i += 2
		}
		if !havePair {
			nv.Name = "_missing"
			nv.Value = args[i]
			i += 1
		}
	}

	// Reverse named value list
	prev = oldLogContext
	for it := list; it != nil; {
		next := it.next
		it.next = prev
		prev = it
		it = next
	}

	return prev
}

func WithValues(ctx context.Context, args ...interface{}) context.Context {
	return context.WithValue(ctx, logContext, mergeStructuredData(ctx, args))
}
