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
"""Tests for gwinpy.net.dhcp."""

import struct
import unittest
import mock
from gwinpy.net import dhcp


class DhcpTest(unittest.TestCase):

  @mock.patch.object(dhcp.socket, 'socket')
  @mock.patch.object(dhcp, '_OptScan', autospec=True)
  def testGetDhcpOption(self, optscan, socket):
    optscan.return_value = None
    result = dhcp.GetDhcpOption('192.168.0.1', '11:22:33:44:55:66', 102)
    self.assertEqual(None, result)
    optscan.return_value = 'America/Chicago'
    result = dhcp.GetDhcpOption('192.168.0.2', '11:22:33:44:55:66', 101)
    self.assertEqual('America/Chicago', result)
    socket.return_value.recv.side_effect = dhcp.socket.timeout
    result = dhcp.GetDhcpOption(
        '192.168.0.2',
        '11:22:33:44:55:66',
        101,
        server_addr='10.0.0.1',
        socket_timeout=5)
    socket.return_value.sendto.assert_called_with(mock.ANY, ('10.0.0.1', 67))
    socket.return_value.settimeout.assert_called_with(5)
    self.assertEqual(None, result)
    # bad mac
    result = dhcp.GetDhcpOption(
        '192.168.0.2', None, 101, server_addr='10.0.0.1', socket_timeout=5)
    # bad ip
    self.assertEqual(None, result)
    result = dhcp.GetDhcpOption(
        None,
        '11:22:33:44:55:66',
        101,
        server_addr='10.0.0.1',
        socket_timeout=5)
    self.assertEqual(None, result)

  def testOptScan(self):
    options = struct.pack('BBBB', 12, 2, 10, 13)
    options += struct.pack('BBB', 40, 1, 1)
    options += struct.pack('BBBBB', 120, 3, 8, 28, 15)
    options += struct.pack('B', 255)
    result = dhcp._OptScan(options, 120)
    self.assertEqual(result, b'\x08\x1c\x0f')
    result = dhcp._OptScan(options, 121)
    self.assertEqual(result, None)

  def testZeroFill(self):
    result = list(dhcp._ZeroFill(10))
    self.assertEqual(len(result), 10)
    for i in result:
      self.assertEqual(b'\x00', i)


if __name__ == '__main__':
  unittest.main()
