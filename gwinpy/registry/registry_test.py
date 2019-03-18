# Copyright 2019 Google Inc. All Rights Reserved.
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
"""Tests for gwinpy.registry."""

import sys
import unittest
from gwinpy.registry import registry
import mock


class RegistryTest(unittest.TestCase):

  def setUp(self):
    self.winreg = mock.Mock()
    self.winreg.KEY_READ = 1
    self.winreg.KEY_WRITE = 2
    sys.modules['_winreg'] = self.winreg
    super(RegistryTest, self).setUp()
    self.reg = registry.Registry(root_key='HKLM')

  def testOpenSubKeyCreate(self):
    self.reg._OpenSubKey(r'SOFTWARE\Test', create=True)
    self.assertTrue(self.winreg.CreateKeyEx.called)

  def testOpenSubKeyOpen(self):
    self.reg._OpenSubKey(r'SOFTWARE\Test', create=False)
    self.assertTrue(self.winreg.OpenKey.called)

  def testOpenSubKeyFail(self):
    registry.WindowsError = Exception
    err = registry.RegistryError('Test', errno=2)
    self.winreg.CreateKeyEx.side_effect = err
    self.assertRaises(
        registry.RegistryError,
        self.reg._OpenSubKey,
        r'SOFTWARE\Test',
        create=True)

  def testGetKeyValue(self):
    self.winreg.QueryValueEx.return_value = ['1.0']
    result = self.reg.GetKeyValue(r'SOFTWARE\Test', 'Release')
    self.assertEqual('1.0', result)

  def testSetKeyValue(self):
    self.assertRaises(
        registry.RegistryError,
        self.reg.SetKeyValue,
        r'SOFTWARE\Test',
        'Release',
        '1.0',
        key_type='REG_FOO')

  def testRemoveKeyValue(self):
    # Variable definition
    registry.RegistryError = Exception
    self.winreg.DeleteValue.side_effect = registry.WindowsError
    self.assertRaises(
        registry.RegistryError,
        self.reg.RemoveKeyValue,
        r'SOFTWARE\Test',
        'Release')


if __name__ == '__main__':
  unittest.main()
