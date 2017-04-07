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
	"os"
	"strconv"
)

type Level int

const (
	LevelDebug Level = iota
	LevelInfo
	LevelNotice
	LevelWarning
	LevelError
	LevelPanic
	LevelFatal
)

func (lvl Level) String() string {
	switch lvl {
	case LevelFatal:
		return "fatal"
	case LevelPanic:
		return "panic"
	case LevelError:
		return "error"
	case LevelWarning:
		return "warning"
	case LevelNotice:
		return "notice"
	case LevelInfo:
		return "info"
	case LevelDebug:
		return "debug"
	default:
		os.Stderr.WriteString("Invalid log level: ")
		os.Stderr.WriteString(strconv.Itoa(int(lvl)))
		os.Stderr.WriteString("\n")
		os.Exit(100)
		return "invalid"
	}
}
