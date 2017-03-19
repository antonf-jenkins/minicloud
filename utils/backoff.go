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

import "time"

type Backoff struct {
	start, end time.Time
	nextWait   time.Duration
}

func NewBackoff(firstWait, timeout time.Duration) *Backoff {
	now := time.Now()
	return &Backoff{
		start:    now,
		end:      now.Add(timeout),
		nextWait: firstWait,
	}
}

func (b *Backoff) Wait() bool {
	now := time.Now()
	if now.After(b.end) {
		return false
	}

	wait := b.nextWait
	b.nextWait *= 2
	if now.Add(wait).After(b.end) {
		wait = b.end.Sub(now)
	}

	time.Sleep(wait)
	return true
}
