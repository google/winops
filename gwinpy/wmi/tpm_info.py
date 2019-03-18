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
"""Classes to enumerate TPM data from WMI."""

import logging
from gwinpy.wmi import wmi_query


class TpmInfo(object):
  """Query TPM data in WMI."""

  def __init__(self):
    self.wmi = wmi_query.WMIQuery(namespace=r'root\cimv2\security\microsofttpm')

  def IsActivated(self):
    """Whether the TPM is currently activated.

    Returns:
      True/False for TPM activated; None for query failure.
    """
    query = 'Select IsActivated_InitialValue from Win32_Tpm'
    results = self.wmi.Query(query)
    if results:
      logging.debug('Win32_Tpm/IsActivated_InitialValue: %s',
                    str(results[0].IsActivated_InitialValue))
      return results[0].IsActivated_InitialValue
    logging.warning('No results for %s.', query)
    return None

  def IsEnabled(self):
    """Whether the TPM is currently enabled.

    Returns:
      True/False for TPM enabled; None for query failure.
    """
    query = 'Select IsEnabled_InitialValue from Win32_Tpm'
    results = self.wmi.Query(query)
    if results:
      logging.debug('Win32_Tpm/IsEnabled_InitialValue: %s',
                    str(results[0].IsEnabled_InitialValue))
      return results[0].IsEnabled_InitialValue
    logging.warning('No results for %s.', query)
    return None

  def IsOwned(self):
    """Whether the TPM is currently owned.

    Returns:
      True/False for TPM ownership; None for query failure.
    """
    query = 'Select IsOwned_InitialValue from Win32_Tpm'
    results = self.wmi.Query(query)
    if results:
      logging.debug('Win32_Tpm/IsOwned_InitialValue: %s',
                    str(results[0].IsOwned_InitialValue))
      return results[0].IsOwned_InitialValue
    logging.warning('No results for %s.', query)
    return None

  def TpmPresent(self):
    """Queries the local host for presence of a TPM device.

    Returns:
      True if device found, else False
    """
    query = 'Select * from Win32_Tpm'
    results = self.wmi.Query(query)
    if len(results):  # pylint: disable=g-explicit-length-test
      return True
    return False

  def TpmSpec(self):
    """Queries the local TPM specification.

    Returns:
      The TPM SpecVersion string, or None.
    """
    query = 'Select SpecVersion from Win32_Tpm'
    results = self.wmi.Query(query)
    if results:
      logging.debug('Win32_Tpm/SpecVersion: %s', results[0].SpecVersion.strip())
      return results[0].SpecVersion.strip()
    logging.warning('No results for %s.', query)
    return None

  def TpmVersion(self):
    """Queries the local TPM device version.

    Returns:
      The TPM version string, or None.
    """
    query = 'Select ManufacturerVersion from Win32_Tpm'
    results = self.wmi.Query(query)
    if results:
      logging.debug('Win32_Tpm/ManufacturerVersion: %s',
                    results[0].ManufacturerVersion.strip())
      return results[0].ManufacturerVersion.strip()
    logging.warning('No results for %s.', query)
    return None
