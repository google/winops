# Copyright 2022 Google Inc. All Rights Reserved.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#      http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.
"""Tests for gwinpy.wmi.process_info."""

import unittest
import mock
from gwinpy.wmi import process_info


class ProcessInfoTest(unittest.TestCase):

  @mock.patch(
      'gwinpy.wmi.wmi_query.WMIQuery', autospec=True)
  def setUp(self, _):
    self.processinfo = process_info.ProcessInfo()
    super(ProcessInfoTest, self).setUp()

  def testProcessNames(self):
    self.processinfo.wmi.Query.return_value = [mock.Mock(Name='python.exe')]
    self.assertEqual(self.processinfo.ProcessNames(), ['python.exe'])
    self.processinfo.wmi.Query.return_value = None
    self.assertIsNone(self.processinfo.ProcessNames())

  def testProcessRunning(self):
    self.processinfo.wmi.Query.return_value = [mock.Mock(Name='python.exe')]
    self.assertEqual(self.processinfo.ProcessRunning('python.exe'), True)
    self.processinfo.wmi.Query.return_value = ''
    self.assertEqual(self.processinfo.ProcessRunning('not_python.exe'), False)

if __name__ == '__main__':
  unittest.main()
