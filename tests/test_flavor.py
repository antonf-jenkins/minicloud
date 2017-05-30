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

from tests import base
from tests import settings
from tests import utils
from tests.utils import mixins

NAME_BASE = utils.random_name('test.flavor.')


class FlavorTest(base.TestCase, mixins.FlavorMixin):
    @classmethod
    def tearDownClass(cls):
        resp = cls.session.get('/flavors')
        assert resp.status_code == 200
        for flavor in resp.json():
            if not flavor['Name'].startswith(NAME_BASE):
                continue
            cls.cleanup_flavor(flavor=flavor,
                               timeout=settings.COMMON_TIMEOUT)
            cls().delete_entity(f'/flavors/{flavor["Id"]}')

    def test_list(self):
        resp = self.session.get('/flavors')
        self.assertEqual(resp.status_code, 200)
        self.assertIsInstance(resp.json(), list)

    def test_create_get(self):
        flavor_name = utils.random_name(NAME_BASE)
        flavor_id = self._create_flavor(flavor_name, 2, 1024)
        flavor = self._get_flavor(flavor_id)
        self.assertEqual(flavor['Name'], flavor_name)
        self.assertEqual(flavor['NumCPUs'], 2)
        self.assertEqual(flavor['RAM'], 1024)

    def test_create_update_get(self):
        new_flavor_name = utils.random_name(NAME_BASE)
        flavor_id = self._create_flavor(utils.random_name(NAME_BASE), 1, 64)
        resp = self.session.put(f'/flavors/{flavor_id}', json={
            'Name': new_flavor_name,
            'NumCPUs': 96,
            'RAM': 512,
        })
        self.assertEqual(resp.status_code, 204)
        flavor = self._get_flavor(flavor_id)
        self.assertEqual(flavor['Name'], new_flavor_name)
        self.assertEqual(flavor['NumCPUs'], 96)
        self.assertEqual(flavor['RAM'], 512)

    # TODO: negative tests
