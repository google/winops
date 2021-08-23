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
"""Tests for gwinpy.wmi.tpm_info."""

import unittest
import mock
from gwinpy.wmi import tpm_info


class TpmInfoTest(unittest.TestCase):

  @mock.patch(
      'gwinpy.wmi.wmi_query.WMIQuery', autospec=True)
  def setUp(self, _):
    self.tpm = tpm_info.TpmInfo()

  def testIsActivated(self):
    self.tpm.wmi.Query.return_value = [mock.Mock(IsActivated_InitialValue=True)]
    self.assertEqual(self.tpm.IsActivated(), True)
    self.tpm.wmi.Query.return_value = None
    self.assertEqual(self.tpm.IsActivated(), None)

  def testIsEnabled(self):
    self.tpm.wmi.Query.return_value = [mock.Mock(IsEnabled_InitialValue=False)]
    self.assertEqual(self.tpm.IsEnabled(), False)
    self.tpm.wmi.Query.return_value = None
    self.assertEqual(self.tpm.IsEnabled(), None)

  def testIsOwned(self):
    self.tpm.wmi.Query.return_value = [mock.Mock(IsOwned_InitialValue=True)]
    self.assertEqual(self.tpm.IsOwned(), True)
    self.tpm.wmi.Query.return_value = None
    self.assertEqual(self.tpm.IsOwned(), None)

  def testTpmPresent(self):
    self.tpm.wmi.Query.return_value = [mock.Mock()]
    self.assertTrue(self.tpm.TpmPresent())
    self.tpm.wmi.Query.return_value = []
    self.assertFalse(self.tpm.TpmPresent())

  def testTpmSpec(self):
    self.tpm.wmi.Query.return_value = [mock.Mock(SpecVersion='1.2')]
    self.assertEqual(self.tpm.TpmSpec(), '1.2')
    self.tpm.wmi.Query.return_value = None
    self.assertEqual(self.tpm.TpmSpec(), None)

  def testTpmVersion(self):
    self.tpm.wmi.Query.return_value = [mock.Mock(ManufacturerVersion='2.81')]
    self.assertEqual(self.tpm.TpmVersion(), '2.81')
    self.tpm.wmi.Query.return_value = None
    self.assertEqual(self.tpm.TpmVersion(), None)


if __name__ == '__main__':
  unittest.main()
