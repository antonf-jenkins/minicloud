# This file is part of the MiniCloud project.
# Copyright (C) 2017 Anton Frolov <frolov.anton@gmail.com>
#
# This program is free software: you can redistribute it and/or modify
# it under the terms of the GNU Affero General Public License as
# published by the Free Software Foundation, either version 3 of the
# License, or (at your option) any later version.
#
# This program is distributed in the hope that it will be useful,
# but WITHOUT ANY WARRANTY; without even the implied warranty of
# MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
# GNU Affero General Public License for more details.
#
# You should have received a copy of the GNU Affero General Public License
# along with this program.  If not, see <http://www.gnu.org/licenses/>.
import os

MINICLOUD_IP = os.environ.get('MINICLOUD_IP', '127.0.0.1')
MINICLOUD_PORT = os.environ.get('MINICLOUD_PORT', '1959')
MINICLOUD_API = os.environ.get('MINICLOUD_API',
                               f'http://{MINICLOUD_IP}:{MINICLOUD_PORT}')
COMMON_TIMEOUT = float(os.environ.get('COMMON_TIMEOUT', 120))
TEST_IMAGE_PATH = os.environ.get('TEST_IMAGE_PATH', __file__)
