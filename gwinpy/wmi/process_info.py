# Copyright 2022 Google Inc. All Rights Reserved.
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
"""Classes to enumerate process(es) data from WMI."""

import logging
from typing import List, Optional

from gwinpy.wmi import wmi_query


class ProcessInfo(object):
  """Query process data in WMI."""

  def __init__(self):
    self.wmi = wmi_query.WMIQuery()

  def ProcessNames(self) -> Optional[List[str]]:
    """Query WMI for names of all running processes.

    Returns:
      Array of all running process names.
    """
    processes = []
    query = 'Select Name from Win32_Process'
    results = self.wmi.Query(query)
    if results:
      for result in results:
        processes.append(result.Name)
      logging.debug('Win32_Process found running processes: %s', processes)
      return processes
    logging.warning('No results for %s.', query)
    return None

  def ProcessRunning(self, name: str) -> bool:
    """Whether the specified process exists.

    Args:
      name: The process to query via WMI.

    Returns:
      True/False for process name found
    """
    query = f'Select Name from Win32_Process WHERE Name LIKE "%{name}%"'
    results = self.wmi.Query(query)
    try:
      logging.debug('Win32_Process/Name: %s', str(results[0].Name))
      return True
    except (AttributeError, IndexError):
      logging.warning('No results for %s', query)
      return False
