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
"""Tests for gwinpy.wmi.net_info."""

import unittest
import mock
from gwinpy.wmi import net_info


class NetInfoTest(unittest.TestCase):

  @mock.patch(
      'gwinpy.wmi.wmi_query.WMIQuery', autospec=True)
  def setUp(self, _):
    self.netinfo = net_info.NetInfo(poll=False)
    self.mock_ip1 = mock.Mock()
    self.mock_ip1.IPAddress = None
    self.mock_ip1.default_gateway = None
    self.mock_ip2 = mock.Mock()
    self.mock_ip2.IPAddress = None
    self.mock_ip2.default_gateway = '2620:0::100'
    self.mock_ip3 = mock.Mock()
    self.mock_ip3.IPAddress = None
    self.mock_ip3.default_gateway = '172.25.100.1'
    self.mock_ip4 = mock.Mock()
    self.mock_ip4.IPAddress = None
    self.mock_ip4.default_gateway = '10.1.10.2'

  def testCheckIfIpv4Address(self):
    self.assertFalse(self.netinfo._CheckIfIpv4Address('foo'))
    self.assertFalse(self.netinfo._CheckIfIpv4Address('192.168'))
    self.assertFalse(self.netinfo._CheckIfIpv4Address('2620:0::100'))
    self.assertTrue(self.netinfo._CheckIfIpv4Address('127.0.0.1'))

  def testDefaultGateways(self):
    self.netinfo._interfaces = [self.mock_ip1, self.mock_ip2, self.mock_ip3]
    self.assertEqual(
        ['2620:0::100', '172.25.100.1'],
        self.netinfo.DefaultGateways(v4_only=False))
    self.assertEqual(
        ['172.25.100.1'], self.netinfo.DefaultGateways(v4_only=True))

  def testDescriptions(self):
    self.mock_ip1.description = 'Intel(R) 82579LM Gigabit Network Connection'
    self.mock_ip2.description = None
    self.mock_ip3.description = None
    self.netinfo._interfaces = [self.mock_ip1, self.mock_ip2, self.mock_ip3]
    self.assertEqual(['Intel(R) 82579LM Gigabit Network Connection'],
                     self.netinfo.Descriptions())

  def testDhcpServers(self):
    self.mock_ip1.dhcp_server = None
    self.mock_ip2.dhcp_server = '172.16.0.1'
    self.mock_ip3.dhcp_server = None
    self.netinfo._interfaces = [self.mock_ip1, self.mock_ip2, self.mock_ip3]
    self.assertEqual(['172.16.0.1'], self.netinfo.DhcpServers())

  def testDnsDomains(self):
    self.mock_ip1.dns_domain = None
    self.mock_ip2.dns_domain = 'google.com'
    self.mock_ip3.dns_domain = None
    self.netinfo._interfaces = [self.mock_ip1, self.mock_ip2, self.mock_ip3]
    self.assertEqual(['google.com'], self.netinfo.DnsDomains())

  def testGetNetConfigsDescription(self):
    self.mock_ip1.Description = None
    self.mock_ip2.Description = 'Intel(R) 82579LM Gigabit Network Connection'
    self.mock_ip3.Description = None
    self.netinfo._wmi.Query.return_value = [
        self.mock_ip1, self.mock_ip2, self.mock_ip3
    ]
    self.netinfo._GetNetConfigs()
    self.assertEqual(['Intel(R) 82579LM Gigabit Network Connection'],
                     self.netinfo.Descriptions())

  def testGetNetConfigsDhcp(self):
    self.mock_ip1.DHCPServer = None
    self.mock_ip2.DHCPServer = None
    self.mock_ip3.DHCPServer = '172.16.0.1'
    self.netinfo._wmi.Query.return_value = [
        self.mock_ip1, self.mock_ip2, self.mock_ip3
    ]
    self.netinfo._GetNetConfigs()
    self.assertEqual(['172.16.0.1'], self.netinfo.DhcpServers())

  def testGetNetConfigsDns(self):
    self.mock_ip1.DNSDomain = None
    self.mock_ip2.DNSDomain = 'google.com'
    self.mock_ip3.DNSDomain = None
    self.netinfo._wmi.Query.return_value = [
        self.mock_ip1, self.mock_ip2, self.mock_ip3
    ]
    self.netinfo._GetNetConfigs()
    self.assertEqual(['google.com'], self.netinfo.DnsDomains())

  def testGetNetConfigsGateway(self):
    self.mock_ip1.DefaultIPGateway = '10.1.10.1'
    self.mock_ip2.DefaultIPGateway = None
    self.mock_ip3.DefaultIPGateway = None
    self.netinfo._wmi.Query.return_value = [
        self.mock_ip1, self.mock_ip2, self.mock_ip3
    ]
    self.netinfo._GetNetConfigs()
    self.assertEqual(['10.1.10.1'], self.netinfo.DefaultGateways())

  def testGetNetConfigsIps(self):
    self.mock_ip1.IPAddress = None
    self.mock_ip2.IPAddress = None
    self.mock_ip3.IPAddress = ['192.168.0.1']
    self.netinfo._wmi.Query.return_value = [
        self.mock_ip1, self.mock_ip2, self.mock_ip3
    ]
    self.netinfo._GetNetConfigs()
    self.assertEqual(['192.168.0.1'], self.netinfo.IpAddresses())

  def testGetNetConfigsMacs(self):
    self.mock_ip1.MACAddress = None
    self.mock_ip2.MACAddress = '01:23:45:67:89:ab'
    self.mock_ip3.MACAddress = None
    self.netinfo._wmi.Query.return_value = [
        self.mock_ip1, self.mock_ip2, self.mock_ip3
    ]
    self.netinfo._GetNetConfigs()
    self.assertEqual(['01:23:45:67:89:ab'], self.netinfo.MacAddresses())

  def testGetNetConfigsQueries(self):
    # all query and data return
    self.mock_ip1.IPAddress = ['192.168.0.1']
    self.netinfo._wmi.Query.return_value = [self.mock_ip1, self.mock_ip2]
    self.netinfo._GetNetConfigs(active_only=False)
    self.netinfo._wmi.Query.assert_called_with(
        'Select * from Win32_NetworkAdapterConfiguration')
    self.assertEqual(len(self.netinfo._interfaces), 2)
    self.netinfo._interfaces = []
    # active query and empty return
    self.netinfo._wmi.Query.return_value = None
    self.netinfo._GetNetConfigs(active_only=True)
    self.netinfo._wmi.Query.assert_called_with(
        'Select * from Win32_NetworkAdapterConfiguration where IPEnabled=true '
        'and DHCPEnabled=true and DNSDomain is not null')
    self.assertEqual(self.netinfo._interfaces, [])

  @mock.patch.object(net_info.subprocess, 'Popen', autospec=True)
  def testGetPtrRecord(self, subproc):
    # hit
    subproc.return_value.communicate.return_value = (
        '8.8.8.in-addr.arpa      name = google-public-dns-a.google.com\n', '')
    result = self.netinfo._GetPtrRecord('8.8.8.8', domain='.google.com')
    subproc.assert_called_with(
        'nslookup -type=PTR 8.8.8.8', stdout=-1, stderr=-1)
    self.assertEqual(result, 'google-public-dns-a.google.com')
    # miss
    subproc.return_value.communicate.return_value = (
        '24.183.139.98.in-addr.arpa      name = ir2.fp.vip.bf1.yahoo.com\n', '')
    result = self.netinfo._GetPtrRecord('98.139.183.24', domain='.google.com')
    self.assertEqual(result, None)
    # nxdomain
    subproc.return_value.communicate.return_value = (
        r'*** UnKnown can\'t find 1.1.1.1.in-addr.arpa.: '
        'Non-existent domain', '')
    result = self.netinfo._GetPtrRecord('1.1.1.1')
    self.assertEqual(result, None)

  def testIpAddresses(self):
    self.mock_ip1.ip_address = None
    self.mock_ip2.ip_address = '192.168.0.1'
    self.mock_ip3.ip_address = None
    self.netinfo._interfaces = [self.mock_ip1, self.mock_ip2, self.mock_ip3]
    self.assertEqual(['192.168.0.1'], self.netinfo.IpAddresses())

  def testMacAddresses(self):
    self.mock_ip1.mac_address = None
    self.mock_ip2.mac_address = None
    self.mock_ip3.mac_address = '01:23:45:67:89:ab'
    self.netinfo._interfaces = [self.mock_ip1, self.mock_ip2, self.mock_ip3]
    self.assertEqual(['01:23:45:67:89:ab'], self.netinfo.MacAddresses())


if __name__ == '__main__':
  unittest.main()
