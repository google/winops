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
"""Tests for gwinpy.wmi.hw_info."""

import unittest
import mock
from gwinpy.wmi import hw_info


class DeviceIdTest(unittest.TestCase):

  def testStringify(self):
    dev_id = hw_info.DeviceId()
    self.assertEqual(str(dev_id), '')
    dev_id = hw_info.DeviceId(ven='8086')
    self.assertEqual(str(dev_id), '8086')
    dev_id = hw_info.DeviceId(ven='8086', dev='0C5A')
    self.assertEqual(str(dev_id), '8086-0C5A')
    dev_id = hw_info.DeviceId(ven='8086', dev='0C5A', subsys='02361028')
    self.assertEqual(str(dev_id), '8086-0C5A-02361028')
    dev_id = hw_info.DeviceId(
        ven='8086', dev='0C5A', subsys='02361028', rev='02')
    self.assertEqual(str(dev_id), '8086-0C5A-02361028-02')


class HwInfoTest(unittest.TestCase):

  @mock.patch(
      'gwinpy.wmi.wmi_query.WMIQuery', autospec=True)
  def setUp(self, _):
    self.hwinfo = hw_info.HWInfo()

  def testBiosSerial(self):
    self.hwinfo.wmi.Query.return_value = [mock.Mock(SerialNumber='12345')]
    self.assertEqual(self.hwinfo.BiosSerial(), '12345')
    self.hwinfo.wmi.Query.return_value = None
    self.assertIsNone(self.hwinfo.BiosSerial())

  def testBIOSVersion(self):
    self.hwinfo.wmi.Query.return_value = [mock.Mock(SMBIOSBIOSVersion='12345')]
    self.assertEqual(self.hwinfo.BIOSVersion(), '12345')
    self.hwinfo.wmi.Query.return_value = None
    self.assertIsNone(self.hwinfo.BIOSVersion())

  def testChassisType(self):
    self.hwinfo.wmi.Query.return_value = [mock.Mock(chassistypes=['10'])]
    self.assertEqual(self.hwinfo.ChassisType(), '10')

  def testComputerSystemManufacturer(self):
    self.hwinfo.wmi.Query.return_value = [mock.Mock(Manufacturer='Dell ')]
    self.assertEqual(self.hwinfo.ComputerSystemManufacturer(), 'Dell')
    self.hwinfo.wmi.Query.return_value = None
    self.assertIsNone(self.hwinfo.ComputerSystemManufacturer())

  def testComputerSystemModel(self):
    self.hwinfo.wmi.Query.return_value = [
        mock.Mock(Model='HP Z620 Workstation')
    ]
    self.assertEqual(self.hwinfo.ComputerSystemModel(), 'HP Z620 Workstation')
    self.hwinfo.wmi.Query.return_value = None
    self.assertIsNone(self.hwinfo.ComputerSystemModel())

  def testHDDSerial(self):
    self.hwinfo.wmi.Query.return_value = [mock.Mock(SerialNumber='12345')]
    self.assertEqual(self.hwinfo.HDDSerial(), '12345')
    self.hwinfo.wmi.Query.return_value = None
    self.assertIsNone(self.hwinfo.HDDSerial())

  def testIsLaptop(self):
    laptop_types = [8, 9, 10, 11, 14]
    with mock.patch.object(
        self.hwinfo, 'ChassisType', autospec=True) as mock_cha:
      mock_cha.return_value = 3
      self.assertFalse(self.hwinfo.IsLaptop())
      for chassis_type in laptop_types:
        mock_cha.return_value = chassis_type
        self.assertTrue(self.hwinfo.IsLaptop())

  def testIsVirtualMachine(self):
    with mock.patch.object(
        self.hwinfo, 'ComputerSystemModel', autospec=True) as model:
      model.return_value = 'Parallels Virtual Platform'
      self.assertTrue(self.hwinfo.IsVirtualMachine())
      model.return_value = 'VMWARE Virtual Platform'
      self.assertTrue(self.hwinfo.IsVirtualMachine())
      model.return_value = 'Virtual Machine'
      self.assertTrue(self.hwinfo.IsVirtualMachine())
      model.return_value = 'HP Z620 Workstation'
      self.assertFalse(self.hwinfo.IsVirtualMachine())

  def testIsOnBattery(self):
    self.hwinfo.wmi.Query.return_value = [mock.Mock(BatteryStatus=1)]
    self.assertTrue(self.hwinfo.IsOnBattery())
    self.hwinfo.wmi.Query.return_value = [
        mock.Mock(BatteryStatus=2),
        mock.Mock(BatteryStatus=2),
        mock.Mock(BatteryStatus=13)
    ]
    self.assertTrue(self.hwinfo.IsOnBattery())
    self.hwinfo.wmi.Query.return_value = [mock.Mock(BatteryStatus=2)]
    self.assertFalse(self.hwinfo.IsOnBattery())
    self.hwinfo.wmi.Query.return_value = []
    self.assertFalse(self.hwinfo.IsOnBattery())
    self.hwinfo.wmi.Query.return_value = None
    self.assertFalse(self.hwinfo.IsOnBattery())

  def testLenovoSystemModel(self):
    self.hwinfo.wmi.Query.return_value = [mock.Mock(Version='ThinkPad T430s')]
    self.assertEqual(self.hwinfo.LenovoSystemModel(), 'ThinkPad T430s')
    self.hwinfo.wmi.Query.return_value = None
    self.assertIsNone(self.hwinfo.LenovoSystemModel())

  def testMacAddresses(self):
    self.hwinfo.wmi.Query.return_value = iter([
        mock.Mock(MacAddress='AA:BB:CC:DD:EE:FF'),
        mock.Mock(MacAddress='11:22:33:44:55:66')
    ])
    result = self.hwinfo.MacAddresses()
    self.assertIn('AA:BB:CC:DD:EE:FF', result)
    self.assertIn('11:22:33:44:55:66', result)
    self.assertFalse('PCI' in self.hwinfo.wmi.Query.call_args[0][0])
    self.hwinfo.wmi.Query.reset_mock()
    self.hwinfo.MacAddresses(pci_only=True)
    self.assertTrue('PCI' in self.hwinfo.wmi.Query.call_args[0][0])

  def testPciDevices(self):
    dev_str = r'PCI\VEN_8086&DEV_1E10&SUBSYS_21FB17AA&REV_C4\3&E89B380&0&E0'
    self.hwinfo.wmi.Query.return_value = [mock.Mock(DeviceID=dev_str)]
    dev_list = self.hwinfo.PciDevices()
    self.assertEqual(dev_list[0].ven, '8086')
    self.assertEqual(dev_list[0].dev, '1E10')
    self.assertEqual(dev_list[0].subsys, '21FB17AA')
    self.assertEqual(dev_list[0].rev, 'C4')

  def testPnpDevices(self):
    self.hwinfo.wmi.Query.return_value = [mock.Mock(Caption='Device A'),
                                          mock.Mock(spec=[]),
                                          mock.Mock(Caption='Device C'),]
    results = self.hwinfo.PnpDevices()
    captions = [dev.caption for dev in results]
    self.hwinfo.wmi.Query.assert_called_with('Select * from Win32_PnPEntity')
    self.assertTrue('Device A' in captions)
    self.assertTrue('Device C' in captions)
    results = self.hwinfo.PnpDevices('some_device_id')
    self.hwinfo.wmi.Query.assert_called_with(
        'Select * from Win32_PnPEntity where DeviceID="some_device_id"')

  def testSmbiosUuid(self):
    self.hwinfo.wmi.Query.return_value = [mock.Mock(UUID='12345')]
    self.assertEqual(self.hwinfo.SmbiosUuid(), '12345')
    self.hwinfo.wmi.Query.return_value = None
    self.assertEqual(self.hwinfo.SmbiosUuid(), None)

  def testUsbDevices(self):
    dev1 = (r'\\WKS\root\cimv2:Win32_PnPEntity.DeviceID'
            r'="USB\\VID_1050&PID_0211\\6&1A571698&0&2"')
    dev2 = (r'\\WKS\root\cimv2:Win32_PnPEntity.DeviceID'
            r'="BTH\\MS_BTHBRB\\7&11E06946&0&1"')
    with mock.patch.object(self.hwinfo, 'PnpDevices') as mock_pnp:
      mock_pnp.side_effect = [['Device 1'], ['Device 2']]
      self.hwinfo.wmi.Query.return_value = [mock.Mock(Dependent=dev1),
                                            mock.Mock(spec=[]),
                                            mock.Mock(Dependent=dev2),]
      results = self.hwinfo.UsbDevices()
      self.hwinfo.wmi.Query.assert_called_with(
          'Select * from Win32_USBControllerDevice')
      mock_pnp.assert_has_calls([
          mock.call(device_id=r'USB\\VID_1050&PID_0211\\6&1A571698&0&2'),
          mock.call(device_id=r'BTH\\MS_BTHBRB\\7&11E06946&0&1')
      ])
      self.assertEqual(results[0], 'Device 1')
      self.assertEqual(results[1], 'Device 2')

  def testVideoControllers(self):
    dev1 = mock.Mock(
        Description='Intel(R) HD Graphics 4000',
        DriverVersion='9.17.10.2843',
        Name='Intel(R) HD Graphics 4000',
        PNPDeviceID=(
            r'PCI\VEN_8086&DEV_0166&SUBSYS_21FB17AA&REV_09\3&E89B380&0&10'),
        AdapterRAM=2214592512)
    dev2 = mock.Mock(
        Description='NVIDIA Quadro K620',
        DriverVersion='10.18.13.5362',
        Name='NVIDIA Quadro K620',
        PNPDeviceID=(
            r'PCI\VEN_10DE&DEV_13BB&SUBSYS_1098103C&REV_A2\4&2A43D483&0&0010'),
        AdapterRAM=2147483648)
    self.hwinfo.wmi.Query.return_value = [dev1, dev2]
    results = self.hwinfo.VideoControllers()
    self.assertEqual(results[0]['description'], 'Intel(R) HD Graphics 4000')
    self.assertEqual(results[1]['driver_version'], '10.18.13.5362')
    self.hwinfo.wmi.Query.return_value = None
    results = self.hwinfo.VideoControllers()
    self.assertEqual(results, [])


if __name__ == '__main__':
  unittest.main()
