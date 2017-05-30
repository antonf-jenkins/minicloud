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
import time
import unittest

from tests import utils


class TestCase(unittest.TestCase):
    session = utils.ApiSession()

    @classmethod
    def wait_404(cls, uri, wait=0.1, timeout=None):
        # TODO: timeout
        while True:
            resp = cls.session.get(uri)
            if resp.status_code == 404:
                return
            assert resp.status_code == 200
            assert resp.json()['State'] == 'deleting'
            time.sleep(wait)

    @classmethod
    def delete_and_wait(cls, uri, timeout=None):
        resp = cls.session.delete(uri)
        assert resp.status_code in (204, 404)
        cls.wait_404(uri, timeout=timeout)

    @classmethod
    def cleanup_project(cls, project=None, project_id=None, timeout=None):
        if project_id is not None:
            resp = cls.session.get(f'/projects/{project_id}')
            assert resp.status_code == 200
            project = resp.json()

        server_ids = project['ServerIds'] or []
        disk_ids = project['DiskIds'] or []
        image_ids = project['ImageIds'] or []

        for server_id in server_ids:
            cls.delete_and_wait(f'/servers/{server_id}', timeout=timeout)

        for disk_id in disk_ids:
            cls.delete_and_wait(f'/disks/{disk_id}', timeout=timeout)

        for image_id in image_ids:
            cls.delete_and_wait(f'/images/{image_id}', timeout=timeout)

    @classmethod
    def cleanup_flavor(cls, flavor=None, flavor_id=None, timeout=None):
        if flavor_id is not None:
            resp = cls.session.get(f'/flavors/{flavor_id}')
            assert resp.status_code == 200
            flavor = resp.json()
            assert isinstance(flavor, dict)

        server_ids = flavor['ServerIds'] or []

        for server_id in server_ids:
            cls.delete_and_wait(f'/servers/{server_id}', timeout=timeout)

    def assertIsUlid(self, value):
        self.assertIsInstance(value, str)
        self.assertEqual(len(value), 26)
        for letter in value:
            self.assertIn(letter, '0123456789ABCDEFGHJKMNPQRSTVWXYZ')

    def create_entity(self, uri, json):
        resp = self.session.post(uri, json=json)
        self.assertEqual(resp.status_code, 204)
        self.assertIn('x-minicloud-id', resp.headers)
        entity_id = resp.headers['x-minicloud-id']
        self.assertIsUlid(entity_id)
        return entity_id

    def get_entity(self, uri, entity_id):
        self.assertIsUlid(entity_id)
        resp = self.session.get(uri)
        self.assertEqual(resp.status_code, 200)
        entity = resp.json()
        self.assertIsInstance(entity, dict)
        self.assertIn('Id', entity)
        self.assertEqual(entity['Id'], entity_id)
        self.assertIn('State', entity)
        self.assertIn('SchemaVersion', entity)
        self.assertEqual(entity['SchemaVersion'], 1)
        return entity

    def delete_entity(self, uri):
        resp = self.session.delete(uri)
        self.assertEqual(resp.status_code, 204)
