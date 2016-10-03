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
"""Tests for gwinpy.wmi.wmi_query."""

import sys
import unittest
import mock
from gwinpy.wmi import wmi_query


class WmiQueryTest(unittest.TestCase):

  def setUp(self):
    # modules
    ## pythoncom
    self.pythoncom = mock.Mock()
    sys.modules['pythoncom'] = self.pythoncom
    ## pywintypes
    self.pywintypes = mock.Mock()
    self.pywintypes.com_error = Exception
    self.pywintypes.error = Exception
    sys.modules['pywintypes'] = self.pywintypes
    ## win32com
    self.win32com = mock.Mock()
    sys.modules['win32com'] = self.win32com
    server = self.win32com.client.Dispatch.return_value
    self.handler = server.ConnectServer.return_value
    # init
    self.wmi = wmi_query.WMIQuery()

  def testInit(self):
    self.win32com.client.Dispatch.return_value.ConnectServer.side_effect = (
        self.pywintypes.com_error)
    self.assertRaises(wmi_query.WmiError, wmi_query.WMIQuery)

  def testQuery(self):
    query = 'Select SerialNumber from Win32_BIOS'
    self.wmi.Query(query)
    self.handler.ExecQuery.assert_called_with(query)
    self.handler.ExecQuery.side_effect = self.pywintypes.com_error
    self.assertRaises(wmi_query.WmiError, self.wmi.Query, query)

  def testWmiInit(self):
    self.wmi._WmiInit()
    self.assertTrue(self.pythoncom.CoInitialize.called)
    self.assertEqual(self.pythoncom, self.wmi._pythoncom)
    self.assertEqual(self.pywintypes, self.wmi._pywintypes)
    self.assertEqual(self.win32com.client, self.wmi._client)

  def testPythoncomImportErr(self):
    del sys.modules['pythoncom']
    self.assertRaises(wmi_query.WmiError, self.wmi._WmiInit)
    sys.modules['pythoncom'] = self.pythoncom
    self.wmi._WmiInit()

  def testPywintypesImportErr(self):
    del sys.modules['pywintypes']
    self.assertRaises(wmi_query.WmiError, self.wmi._WmiInit)
    sys.modules['pywintypes'] = self.pywintypes
    self.wmi._WmiInit()

  def testWin32comImportErr(self):
    del sys.modules['win32com']
    self.assertRaises(wmi_query.WmiError, self.wmi._WmiInit)
    sys.modules['win32com'] = self.win32com
    self.wmi._WmiInit()


if __name__ == '__main__':
  unittest.main()
