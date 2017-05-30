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
import asserts

from tests import base
from tests import settings
from tests import utils
from tests.utils import mixins

NAME_BASE = utils.random_name('test-project-')


class ProjectTest(base.TestCase, mixins.ProjectMixin):
    @classmethod
    def tearDownClass(cls):
        resp = cls.session.get('/projects')
        asserts.assert_equal(resp.status_code, 200)
        for project in resp.json():
            if not project['Name'].startswith(NAME_BASE):
                continue
            cls.cleanup_project(project=project,
                                timeout=settings.COMMON_TIMEOUT)
            resp = cls.session.delete(f'/projects/{project["Id"]}')
            asserts.assert_equal(resp.status_code, 204)

    def test_list(self):
        resp = self.session.get('/projects')
        asserts.assert_equal(resp.status_code, 200)
        asserts.assert_is_instance(resp.json(), list)

    def test_create_get(self):
        project_name = utils.random_name(NAME_BASE)
        project_id = self._create_project(project_name)
        project = self._get_project(project_id)
        asserts.assert_equal(project['Name'], project_name)

    def test_create_update_get(self):
        new_project_name = utils.random_name(NAME_BASE)
        project_id = self._create_project(utils.random_name(NAME_BASE))
        resp = self.session.put(f'/projects/{project_id}', json={
            'Name': new_project_name
        })
        asserts.assert_equal(resp.status_code, 204)
        project = self._get_project(project_id)
        asserts.assert_equal(project['Name'], new_project_name)

    def test_same_name_rejected(self):
        project_name = utils.random_name(NAME_BASE)
        self._create_project(project_name)
        resp = self.session.post('/projects', json={'Name': project_name})
        asserts.assert_equal(resp.status_code, 409)

    def test_rename_existing_name_rejected(self):
        project_name = utils.random_name(NAME_BASE)
        self._create_project(project_name)
        project_id = self._create_project(utils.random_name(NAME_BASE))
        resp = self.session.put(f'/projects/{project_id}', json={
            'Name': project_name
        })
        asserts.assert_equal(resp.status_code, 409)

    def test_invalid_name_rejected(self):
        resp = self.session.post('/projects', json={'Name': '<html>'})
        asserts.assert_equal(resp.status_code, 400)

    def test_rename_to_invalid_name_rejected(self):
        project_id = self._create_project(utils.random_name(NAME_BASE))
        resp = self.session.put(f'/projects/{project_id}', json={
            'Name': '<html>'
        })
        asserts.assert_equal(resp.status_code, 400)
