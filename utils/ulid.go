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

import (
	"crypto/rand"
	"encoding/hex"
	"github.com/oklog/ulid"
)

var Zero = ulid.ULID{}

func NewULID() ulid.ULID {
	return ulid.MustNew(ulid.Now(), rand.Reader)
}

func RemoveULID(list []ulid.ULID, item ulid.ULID) []ulid.ULID {
	for idx, elem := range list {
		if elem == item {
			listLen := len(list)
			list[idx] = list[listLen-1]
			list = list[:listLen-1]
			return list
		}
	}
	return list
}

func ULIDListsEqual(x []ulid.ULID, y []ulid.ULID) bool {
	if len(x) != len(y) {
		return false
	}
	for i := range x {
		if x[i] != y[i] {
			return false
		}
	}
	return true
}

func ULIDListCopy(x []ulid.ULID) []ulid.ULID {
	y := make([]ulid.ULID, len(x))
	for i, value := range x {
		y[i] = value
	}
	return y
}

func ConvertToUUID(ulid ulid.ULID) string {
	buf := make([]byte, 36)

	hex.Encode(buf[0:8], ulid[0:4])
	buf[8] = '-'
	hex.Encode(buf[9:13], ulid[4:6])
	buf[13] = '-'
	hex.Encode(buf[14:18], ulid[6:8])
	buf[18] = '-'
	hex.Encode(buf[19:23], ulid[8:10])
	buf[23] = '-'
	hex.Encode(buf[24:], ulid[10:])

	return string(buf)
}
