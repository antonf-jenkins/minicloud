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

import "reflect"

func GetFieldValue(structVal interface{}, field string) interface{} {
	structRv := reflect.ValueOf(structVal)
	if structRv.Kind() == reflect.Ptr {
		structRv = structRv.Elem()
	}
	fieldRv := structRv.FieldByName(field)
	if fieldRv.Kind() == reflect.Invalid {
		panic("Invalid field value: " + field)
	}
	return fieldRv.Interface()
}

// Returns interface containing pointer to copy of original struct value
func MakeStructCopy(value interface{}) interface{} {
	origRv := reflect.ValueOf(value)
	if origRv.Kind() == reflect.Ptr {
		origRv = origRv.Elem()
	}
	copyRv := reflect.New(origRv.Type())
	copyRv.Elem().Set(origRv)
	return copyRv.Interface()
}
