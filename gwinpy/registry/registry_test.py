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

import unittest
import mock
import six
from gwinpy.registry import registry

_FAKE_REG_KEY = r'SOFTWARE\Test'
_WINDOWS_DATA_ERROR = OSError('No more data is available')
_WINDOWS_DATA_ERROR.winerror = 259
_WINDOWS_FAKE_ERROR = OSError('Some other error')
_WINDOWS_FAKE_ERROR.winerror = 1337


class RegistryTest(unittest.TestCase):

  def setUp(self):
    super(RegistryTest, self).setUp()
    self.winreg = mock.Mock()
    self.winreg.KEY_READ = 1
    self.winreg.KEY_WRITE = 2
    with mock.patch.object(six.moves, 'winreg', create=True):
      self.reg = registry.Registry(root_key='HKLM')
      self.reg._winreg = self.winreg

  def testOpenSubKeyCreate(self):
    self.reg._OpenSubKey(_FAKE_REG_KEY, create=True)
    self.assertTrue(self.winreg.CreateKeyEx.called)

  def testOpenSubKeyOpen(self):
    self.reg._OpenSubKey(_FAKE_REG_KEY, create=False)
    self.assertTrue(self.winreg.OpenKey.called)

  def testOpenSubKeyFail(self):
    registry.WindowsError = Exception
    err = registry.RegistryError('Test', errno=2)
    self.winreg.CreateKeyEx.side_effect = err
    self.assertRaises(
        registry.RegistryError,
        self.reg._OpenSubKey,
        _FAKE_REG_KEY,
        create=True)

  def testGetKeyValue(self):
    self.winreg.QueryValueEx.return_value = ['1.0']
    result = self.reg.GetKeyValue(_FAKE_REG_KEY, 'Release')
    self.assertEqual('1.0', result)

  def testGetKeyValues(self):
    self.winreg.EnumKey.side_effect = ('Release', _WINDOWS_DATA_ERROR)
    result = self.reg.GetRegKeys(_FAKE_REG_KEY)
    self.assertEqual(['Release'], result)

  def testGetKeyValuesFail(self):
    self.winreg.EnumKey.side_effect = _WINDOWS_FAKE_ERROR
    self.assertRaises(registry.RegistryError, self.reg.GetRegKeys,
                      _FAKE_REG_KEY)

  def testGetKeysAndValues(self):
    self.winreg.EnumValue.side_effect = (('Release', '1.0',
                                          self.winreg.REG_DWORD),
                                         _WINDOWS_DATA_ERROR)
    result = self.reg.GetRegKeysAndValues(_FAKE_REG_KEY)
    self.assertEqual([('Release', '1.0', self.winreg.REG_DWORD)], result)

  def testGetKeysAndValuesFail(self):
    self.winreg.EnumValue.side_effect = _WINDOWS_FAKE_ERROR
    with self.assertRaises(registry.RegistryError):
      self.reg.GetRegKeysAndValues(_FAKE_REG_KEY)

  def testSetKeyValue(self):
    self.assertRaises(
        registry.RegistryError,
        self.reg.SetKeyValue,
        _FAKE_REG_KEY,
        'Release',
        '1.0',
        key_type='REG_FOO')

  def testRemoveKeyValue(self):
    # Variable definition
    registry.RegistryError = Exception
    self.winreg.DeleteValue.side_effect = registry.WindowsError
    self.assertRaises(registry.RegistryError, self.reg.RemoveKeyValue,
                      _FAKE_REG_KEY, 'Release')


if __name__ == '__main__':
  unittest.main()
