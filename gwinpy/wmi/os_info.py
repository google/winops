#!/usr/bin/python
#
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
"""A class to enumerate basic operating system data from WMI."""

import logging
import wmi_query


class OSInfo(object):
  """Query basic operating system data in WMI."""

  def __init__(self, logger=None):
    self.wmi = wmi_query.WMIQuery()
    if logger:
      self.logger = logger
    else:
      self.logger = logging

  def OperatingSystem(self):
    """Get the operating system name from Win32_OperatingSystem.

    Returns:
      The Name string if found; else None.
    """
    query = 'Select Name from Win32_OperatingSystem'
    results = self.wmi.Query(query)
    if results:
      self.logger.debug('Win32_OperatingSystem/Name: %s' %
                        results[0].Name.strip())
      return results[0].Name.strip()
    self.logger.warning('No results for %s.' % query)
    return None

  def OperatingSystemVersion(self):
    """Get the operating system version from Win32_OperatingSystem.

    Returns:
      The version number if found; else None.
    """
    query = 'Select Version from Win32_OperatingSystem'
    results = self.wmi.Query(query)
    if results:
      self.logger.debug('Win32_OperatingSystem/Version: %s' %
                        results[0].Version.strip())
      return results[0].Version.strip()
    self.logger.warning('No results for %s.' % query)
    return None

  def IsServer(self):
    """Check whether the OS is a Windows Server OS.

    Returns:
      True if the machine is running server version of Windows.
    """
    if 'server' in self.OperatingSystem().lower():
      return True
    else:
      return False
