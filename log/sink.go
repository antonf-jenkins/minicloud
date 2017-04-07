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
	"bytes"
	"context"
	"fmt"
	"os"
	"sync"
)

var (
	sink    = make(chan *Message, 100)
	bufPool = sync.Pool{
		New: func() interface{} {
			return &bytes.Buffer{}
		},
	}
)

func Initialize(ctx context.Context) {
	if ctx == nil {
		ctx = context.Background()
	}
	go processMessages(ctx)
}

func Sync() {
	waitCh := make(chan struct{})
	msg := msgPool.Get().(*Message)
	msg.delivered = waitCh
	sink <- msg
	<-waitCh
}

func processMessages(ctx context.Context) {
	for {
		select {
		case msg := <-sink:
			if !msg.Time.IsZero() {
				formatMessage(msg)
			}
			recycleMsg(msg)
		case <-ctx.Done():
			return
		}
	}
}

func formatMessage(msg *Message) {
	// 2006-01-02 15:04:05.999 error   foo:
	buf := bufPool.Get().(*bytes.Buffer)
	level := msg.Level.String()
	buf.WriteString(level)
	for i := 0; i < 8-len(level); i++ {
		buf.WriteByte(' ')
	}
	buf.WriteString(msg.Time.Format("2006-01-02 15:04:05.999"))
	buf.WriteByte(' ')
	buf.WriteString(msg.Logger)
	buf.WriteString(": ")
	buf.WriteString(msg.Message)
	for it := msg.StructuredData; it != nil; it = it.next {
		buf.WriteByte(' ')
		buf.WriteString(it.Name)
		buf.WriteByte('=')
		buf.WriteString(fmt.Sprintf("%v", it.Value))
	}
	buf.WriteByte('\n')
	os.Stderr.Write(buf.Bytes())
	buf.Reset()
	bufPool.Put(buf)
}
