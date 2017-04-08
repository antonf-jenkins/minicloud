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
	"os"
	"sync"
	"time"
)

type Message struct {
	Level          Level
	Time           time.Time
	Logger         string
	Message        string
	StructuredData *NamedValue
	delivered      chan struct{}
}

type Logger interface {
	Debug(ctx context.Context, message string, args ...interface{})
	Info(ctx context.Context, message string, args ...interface{})
	Notice(ctx context.Context, message string, args ...interface{})
	Warn(ctx context.Context, message string, args ...interface{})
	Error(ctx context.Context, message string, args ...interface{})
	Panic(ctx context.Context, message string, args ...interface{})
	Fatal(ctx context.Context, message string, args ...interface{})
}

func New(name string) Logger {
	return &logger{name}
}

var msgPool = sync.Pool{
	New: func() interface{} {
		return &Message{}
	},
}

func recycleMsg(msg *Message) {
	if msg.delivered != nil {
		close(msg.delivered)
		msg.delivered = nil
	}
	msg.Time = time.Time{}
	msg.StructuredData = nil
	msg.Logger = ""
	msg.Message = ""
	msg.Level = LevelDebug
	msgPool.Put(msg)
}

type logger struct {
	name string
}

func (logger *logger) log(ctx context.Context, level Level, message string, args ...interface{}) {
	msg := msgPool.Get().(*Message)
	msg.Time = time.Now().UTC()
	msg.Level = level
	msg.Logger = logger.name
	msg.Message = message
	msg.StructuredData = mergeStructuredData(ctx, args)
	sink <- msg
}

func (logger *logger) Debug(ctx context.Context, message string, args ...interface{}) {
	logger.log(ctx, LevelDebug, message, args...)
}

func (logger *logger) Info(ctx context.Context, message string, args ...interface{}) {
	logger.log(ctx, LevelInfo, message, args...)
}

func (logger *logger) Notice(ctx context.Context, message string, args ...interface{}) {
	logger.log(ctx, LevelNotice, message, args...)
}

func (logger *logger) Warn(ctx context.Context, message string, args ...interface{}) {
	logger.log(ctx, LevelWarning, message, args...)
}

func (logger *logger) Error(ctx context.Context, message string, args ...interface{}) {
	logger.log(ctx, LevelError, message, args...)
}

func (logger *logger) Panic(ctx context.Context, message string, args ...interface{}) {
	logger.log(ctx, LevelError, message, args...)
	panic(message)
}

func (logger *logger) Fatal(ctx context.Context, message string, args ...interface{}) {
	logger.log(ctx, LevelFatal, message, args...)
	Sync()
	os.Exit(1)
}
