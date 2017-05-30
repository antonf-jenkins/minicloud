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

NAME_BASE = utils.random_name('test-image-')


class ImageTest(base.TestCase, mixins.ProjectMixin):
    project_id = None

    @classmethod
    def setUpClass(cls):
        project_name = utils.random_name('test-image-project-')
        cls.project_id = cls()._create_project(project_name)

    @classmethod
    def tearDownClass(cls):
        cls.cleanup_project(project_id=cls.project_id)
        resp = cls.session.delete(f'/projects/{cls.project_id}')
        assert resp.status_code == 204

    def test_create_get(self):
        image_name = utils.random_name(NAME_BASE)
        image_id = self.create_entity('/images', {
            'Name': image_name,
            'ProjectId': self.project_id,
        })
        image = self.get_entity(f'/images/{image_id}', image_id)
        self.assertIn('Name', image)
        self.assertEqual(image['Name'], image_name)
        self.assertIn('ProjectId', image)
        self.assertEqual(image['ProjectId'], self.project_id)
        project = self._get_project(self.project_id)
        self.assertIn(image_id, project['ImageIds'])
