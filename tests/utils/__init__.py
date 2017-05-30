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
import codecs
import os
from urllib import parse

import requests

from tests import settings


class ApiSession(requests.Session):
    def __init__(self):
        super(ApiSession, self).__init__()
        self.headers['Content-Type'] = 'application/json'

    def request(self, method, url, **kwargs):
        full_url = parse.urljoin(settings.MINICLOUD_API, url)
        return super(ApiSession, self).request(method, full_url, **kwargs)


def random_name(base):
    rnd_part = codecs.encode(os.urandom(4), 'hex').decode('ascii')
    return f'{base}{rnd_part}'

