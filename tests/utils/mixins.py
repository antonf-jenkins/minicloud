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


class ProjectMixin(object):
    def _create_project(self, name):
        return self.create_entity('/projects', {'Name': name})

    def _get_project(self, project_id):
        project = self.get_entity(f'/projects/{project_id}', project_id)
        self.assertIn('State', project)
        self.assertIn('Name', project)
        self.assertIn('ImageIds', project)
        self.assertIn('DiskIds', project)
        self.assertIn('ServerIds', project)
        return project


class FlavorMixin(object):
    def _create_flavor(self, name, num_cpus, ram):
        return self.create_entity('/flavors', {
            'Name': name,
            'NumCPUs': num_cpus,
            'RAM': ram,
        })

    def _get_flavor(self, flavor_id):
        flavor = self.get_entity(f'/flavors/{flavor_id}', flavor_id)
        self.assertIn('Name', flavor)
        self.assertIn('NumCPUs', flavor)
        self.assertIn('RAM', flavor)
        self.assertIn('ServerIds', flavor)
        return flavor
