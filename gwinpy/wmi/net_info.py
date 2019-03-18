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
"""Classes to enumerate basic network data from WMI.

Unlike some system elements (eg hardware), network state is prone to changing
regularly and dynamically.  There are also associations between much of the
data: for example, each IP address and MAC are associated with a single NIC.  It
would be chaotic to return the MAC address from one NIC with the IP address
from another.

To resolve these concerns, the network state is polled in batch via a call to
Poll().  This represents a "snapshot" of the current network state.  This avoids
the possibility that multiple consecutive WMI queries will return the interfaces
in different orders, thereby confusing the variable associations.
"""

import logging
import re
import socket
import subprocess
from gwinpy.wmi import wmi_query


class NetInterface(object):
  """Stores all settings for a single interface."""

  def __init__(self,
               default_gateway=None,
               description=None,
               dhcp_server=None,
               dns_domain=None,
               ip_address=None,
               mac_address=None):
    self.default_gateway = default_gateway
    self.description = description
    self.dhcp_server = dhcp_server
    self.dns_domain = dns_domain
    self.ip_address = ip_address
    self.mac_address = mac_address


class NetInfo(object):
  """Query basic network data in WMI."""

  def __init__(self, active_only=True, poll=True):
    self._wmi = wmi_query.WMIQuery()
    self._interfaces = []
    if poll:
      self.Poll(active_only)

  def _CheckIfIpv4Address(self, ip_address):
    """Checks if an input string is an IPv4 address.

    Args:
      ip_address: The input string to check.

    Returns:
      True if the input is an IPv4 address, else False.
    """
    try:
      socket.inet_aton(ip_address)  # pylint:disable=g-socket-inet-aton
    except socket.error:
      return False
    # socket.inet_aton will pad zeroes onto a string like '192.168' when
    # validating, so check to make sure there are four parts in the address
    if len(ip_address.split('.')) == 4:
      return True
    return False

  def DefaultGateways(self, v4_only=False):
    """Get all default gateways from Win32_NetworkAdapterConfiguration.

    Args:
      v4_only: Only store gateways which appear to be valid IPv4 addresses.

    Returns:
      A list of default gateways.
    """
    default_gateways = []
    for interface in self._interfaces:
      if interface.default_gateway:
        gateway = interface.default_gateway
        if v4_only and not self._CheckIfIpv4Address(str(gateway)):
          continue
        default_gateways.append(gateway)
    return default_gateways

  def Descriptions(self):
    """Get all interface descriptions from Win32_NetworkAdapterConfiguration.

    Returns:
      A list of interface descriptions.
    """
    descriptions = []
    for interface in self._interfaces:
      if interface.description:
        descriptions.append(interface.description)
    return descriptions

  def DhcpServers(self):
    """Get all DHCP servers from Win32_NetworkAdapterConfiguration.

    Returns:
      A list of DHCP servers.
    """
    dhcp_servers = []
    for interface in self._interfaces:
      if interface.dhcp_server:
        dhcp_servers.append(interface.dhcp_server)
    return dhcp_servers

  def DnsDomains(self):
    """Get all dns domains from Win32_NetworkAdapterConfiguration.

    Returns:
      A list of dns domains.
    """
    dns_domains = []
    for interface in self._interfaces:
      if interface.dns_domain:
        dns_domains.append(interface.dns_domain)
    return dns_domains

  def _GetNetConfigs(self, active_only=True):
    """Retrieves the network adapter configuration from local NICs.

    Active NICs are defined as interfaces where the nic is enabled, dhcp is
    enabled, and the DNS domain is not null.

    Args:
      active_only: only retrieve configuration from "active" NICs
    """
    query = 'Select * from Win32_NetworkAdapterConfiguration'
    if active_only:
      query += (' where IPEnabled=true and DHCPEnabled=true and'
                ' DNSDomain is not null')
    results = self._wmi.Query(query)
    if results:
      for interface in results:
        found_int = NetInterface()
        if (hasattr(interface, 'DefaultIPGateway') and
            interface.DefaultIPGateway):
          found_int.default_gateway = interface.DefaultIPGateway
        if hasattr(interface, 'Description') and interface.Description:
          found_int.description = interface.Description
        if hasattr(interface, 'DHCPServer') and interface.DHCPServer:
          found_int.dhcp_server = interface.DHCPServer
        if hasattr(interface, 'DNSDomain') and interface.DNSDomain:
          found_int.dns_domain = interface.DNSDomain
        if hasattr(interface, 'IPAddress') and interface.IPAddress:
          found_int.ip_address = interface.IPAddress[0]
        if hasattr(interface, 'MACAddress') and interface.MACAddress:
          found_int.mac_address = interface.MACAddress
        self._interfaces.append(found_int)
    else:
      logging.warning('No results for %s.', query)

  def _GetPtrRecord(self, ip_address, domain='.com'):
    """Gets the DNS PTR record for an IPv4 address.

    Args:
      ip_address: The IP address string to check the PTR record for.
      domain: The parent domain of expected pointer records.

    Returns:
      A string containing the FQDN of the IP address, or None if not found.
    """
    logging.debug('Checking PTR record for %s.', ip_address)
    subproc = subprocess.Popen(
        'nslookup -type=PTR %s' % ip_address,
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE)
    output, unused_error = subproc.communicate()
    hostname = '([a-zA-Z0-9.-]+%s)' % domain.replace('.', r'\.')
    result = re.search(r'name = %s' % hostname, output)
    if result:
      return result.group(1)
    return None

  def Interfaces(self):
    """Get all interfaces.

    Returns:
      All interfaces as a list of NetInterface objects
    """
    return self._interfaces

  def IpAddresses(self):
    """Get all IP Addresses from Win32_NetworkAdapterConfiguration.

    Returns:
      A list of local IP Addresses.
    """
    ip_addresses = []
    for interface in self._interfaces:
      if interface.ip_address:
        ip_addresses.append(interface.ip_address)
    return ip_addresses

  def MacAddresses(self):
    """Get all mac addresses from Win32_NetworkAdapterConfiguration.

    Active NICs are defined as interfaces where the nic is enabled, dhcp is
    enabled, and the DNS domain is not null.

    Returns:
      A list of mac addresses.
    """
    mac_addresses = []
    for interface in self._interfaces:
      if interface.mac_address:
        mac_addresses.append(interface.mac_address)
    return mac_addresses

  def Poll(self, active_only=True):
    """Poll all network interfaces for current state.

    Active NICs are defined as interfaces where the nic is enabled, dhcp is
    enabled, and the DNS domain is not null.

    Args:
      active_only: only retrieve configuration from "active" NICs
    """
    self._interfaces = []
    self._GetNetConfigs(active_only=active_only)
