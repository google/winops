# Copyright 2016 Google Inc. All Rights Reserved.
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
"""Tests for gwinpy.wmi.os_info."""

import unittest
import mock
from gwinpy.wmi import os_info


class OsInfoTest(unittest.TestCase):

  @mock.patch(
      'gwinpy.wmi.wmi_query.WMIQuery', autospec=True)
  def setUp(self, _):
    self.osinfo = os_info.OSInfo()

  def testOperatingSystem(self):
    self.osinfo.wmi.Query.return_value = [mock.Mock(Name='Microsoft Windows 7')]
    self.assertEqual(self.osinfo.OperatingSystem(), 'Microsoft Windows 7')
    self.osinfo.wmi.Query.return_value = None
    self.assertEqual(self.osinfo.OperatingSystem(), None)

  def testOperatingSystemVersion(self):
    self.osinfo.wmi.Query.return_value = [mock.Mock(Version='6.1.7601')]
    self.assertEqual(self.osinfo.OperatingSystemVersion(), '6.1.7601')
    self.osinfo.wmi.Query.return_value = None
    self.assertEqual(self.osinfo.OperatingSystemVersion(), None)

  def testIsServer(self):
    with mock.patch.object(
        self.osinfo, 'OperatingSystem', autospec=True) as osname:
      osname.return_value = 'Microsoft Windows 7 Enterprise'
      self.assertFalse(self.osinfo.IsServer())
      osname.return_value = 'Microsoft Windows Server 2008 R2 Enterprise'
      self.assertTrue(self.osinfo.IsServer())

  def testIsDomainController(self):
    self.osinfo.wmi.Query.return_value = [mock.Mock(DomainRole=1)]
    self.assertFalse(self.osinfo.IsDomainController())
    self.osinfo.wmi.Query.return_value = [mock.Mock(DomainRole=4)]
    self.assertTrue(self.osinfo.IsDomainController())
    self.osinfo.wmi.Query.return_value = [mock.Mock(DomainRole=5)]
    self.assertTrue(self.osinfo.IsDomainController())
    self.osinfo.wmi.Query.return_value = None
    self.assertRaises(os_info.Error, self.osinfo.IsDomainController)

if __name__ == '__main__':
  unittest.main()
