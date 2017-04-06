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
package fsm

import (
	"fmt"
	"github.com/antonf/minicloud/db"
)

type InvalidStateError struct {
	State db.State
}

func (e *InvalidStateError) Error() string {
	return fmt.Sprintf("Invalid state: %s", e.State)
}

type InvalidTransitionError struct {
	From db.State
	To   db.State
}

func (e *InvalidTransitionError) Error() string {
	return fmt.Sprintf("Invalid state transition: %s -> %s", e.From, e.To)
}
