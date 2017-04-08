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
	"context"
	"github.com/antonf/minicloud/config"
	"github.com/antonf/minicloud/db"
)

func Retry(ctx context.Context, operationFn func() error) (err error) {
	retryCount := config.OptRetryCount.Value()
	for i := 0; i < retryCount; i++ {
		if err = operationFn(); err != nil {
			if _, ok := err.(*db.ConflictError); ok {
				logger.Error(ctx, "operation failed, trying again", "error", err, "attempt", i)
				continue
			}
		} else {
			return
		}
	}
	return
}
