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
package ceph

const (
	RbdFeatureLayering      = 1 << 0
	RbdFeatureStripingV2    = 1 << 1
	RbdFeatureExclusiveLock = 1 << 2
	RbdFeatureObjectMap     = 1 << 3
	RbdFeatureFastDiff      = 1 << 4
	RbdFeatureDeepFlatten   = 1 << 5
	RbdFeatureJournaling    = 1 << 6
	RbdFeatureDataPool      = 1 << 7

	RbdFeaturesDefault = RbdFeatureLayering |
		RbdFeatureExclusiveLock |
		RbdFeatureObjectMap |
		RbdFeatureFastDiff |
		RbdFeatureDeepFlatten
)
