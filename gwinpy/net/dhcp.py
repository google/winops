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
"""Functions for interacting with DHCP servers."""

import logging
import random
import socket
import struct

import six


def _BuildPacket(client_addr, client_mac, option):
  """Construct the DHCP Packet.

  https://tools.ietf.org/html/rfc2131#page-8

  Args:
    client_addr: The client IP address as a string.
    client_mac: The client MAC address as a string.
    option: The option number being requested by the user.

  Returns:
    The compiled DHCP packet string.
  """
  ip_addr = None
  packed_mac = b''

  try:
    for i in client_mac.split(':'):
      packed_mac += struct.pack('B', int(i, 16))
  except AttributeError:
    logging.error('Cannot interpret mac address %s.', client_mac)
    return None

  try:
    ip_addr = socket.inet_aton(client_addr)  # pylint:disable=g-socket-inet-aton
  except TypeError:
    pass

  if not ip_addr:
    logging.error('Cannot translate client address %s.', client_addr)
    return None

  xid = random.randint(1, 2**31 - 1)
  data = struct.pack('!BBBBLHH',
                     1,  # message type
                     1,  # hardware type
                     6,  # address length
                     0,  # hops
                     xid,  # tx id
                     0,  # seconds elapsed
                     0,  # flags
                    )
  data += ip_addr
  data += b''.join(_ZeroFill(12))
  data += packed_mac
  data += b''.join(_ZeroFill(202))
  data += struct.pack('BBBB', 99, 130, 83, 99)  # magic cookie
  data += struct.pack('BBBBBB',
                      53,  # message type: dhcp
                      1,  # length
                      8,  # inform type
                      61,  # client identifier
                      7,  # length
                      1,  # hardware type: ethernet
                     )
  data += packed_mac
  data += struct.pack('BBBBB',
                      55,  # parameter request list
                      2,  # length
                      43,  # vendor specific information
                      option,  # user requested option number
                      255,)  # end
  return data


def GetDhcpOption(client_addr,
                  client_mac,
                  option,
                  server_addr='255.255.255.255',
                  socket_timeout=10):
  """Requests a DHCP option (RFC2132) from a DHCP server.

  Args:
    client_addr: The client IP address as a string.
    client_mac: The client MAC address as a string.
    option: The option number being requested by the user.
    server_addr: The DHCP server address to target.
    socket_timeout: How long to wait for a response on port 68.

  Returns:
    The response payload corresponding to the requested option, or None.
  """
  result = None
  option = int(option)
  data = _BuildPacket(client_addr, client_mac, option)

  if not data:
    logging.error('Cannot compile DHCP request.')
    return result

  sock = socket.socket(socket.AF_INET, socket.SOCK_DGRAM)
  sock.setsockopt(socket.SOL_SOCKET, socket.SO_REUSEADDR, 1)
  sock.setsockopt(socket.SOL_SOCKET, socket.SO_BROADCAST, 1)
  sock.bind((client_addr, 68))
  sock.settimeout(socket_timeout)

  sock.sendto(data, (server_addr, 67))
  response = None
  try:
    response = sock.recv(1024)
  except socket.timeout:
    logging.error('Timed out waiting for response.')
  if response:
    result = _OptScan(response[240:], option)
  return result


def _Unpack(options, index):
  if six.PY2:
    return struct.unpack('B', options[index])[0]
  return options[index]


def _OptScan(options, target):
  """Scan the options fields in a DHCP query response.

  The DHCP query response contains a variable series of options packed as:
    [1 byte]: Option number
    [1 byte]: Response length integer
    [X bytes]: Option data of the length specified

  Option 255 terminates the options list.

  https://tools.ietf.org/html/rfc2132

  Args:
    options:  The string containing all DHCP options set in the query response.
    target: The option number we're searcing for.

  Returns:
    The content of the requested option number field, or None if not found.
  """
  i = 0
  while i < len(options):
    number = _Unpack(options, i)
    if number == 255:
      break
    size = _Unpack(options, i + 1)
    i += 2
    if number == target:
      return options[i:i + int(size)]
    i += size
  return None


def _ZeroFill(n):
  """Generate an arbitrary number of zeros for padding."""
  i = 0
  while i < n:
    yield b'\x00'
    i += 1
