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
package db

import (
	"fmt"
	"github.com/oklog/ulid"
	"regexp"
	"strings"
)

type FieldError struct {
	Entity, Field, Message string
}

func (e *FieldError) Error() string {
	return fmt.Sprintf("%s.%s invalid: %s", e.Entity, e.Field, e.Message)
}

func checkFieldRegexp(entity, field, value string, regexp *regexp.Regexp) error {
	if !regexp.MatchString(value) {
		return &FieldError{
			Entity:  entity,
			Field:   field,
			Message: fmt.Sprintf("Field must match regexp: %s", regexp),
		}
	}
	return nil
}

type ConflictError struct {
	Xid string
}

func (e *ConflictError) Error() string {
	return fmt.Sprintf("Conflict committing transaction %s", e.Xid)
}

type NotFoundError struct {
	Entity string
	Id     ulid.ULID
}

func (e *NotFoundError) Error() string {
	return fmt.Sprintf("%s with id %s not found", strings.Title(e.Entity), e.Id)
}
