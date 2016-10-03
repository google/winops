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
"""Class to provide basic WMI query functionality."""


class WmiError(Exception):
  pass


class WMIQuery(object):
  """Facilitates queries against WMI."""

  def __init__(self, host='.', namespace=r'root\cimv2'):
    r"""WMIQuery object initialization.

    Args:
      host: (string) The host to query.  Defaults to localhost.
      namespace: (string) Namespace against which you want to run the query.
        Defaults to root\cimv2.
    """
    self._WmiInit()
    try:
      self.server = self._client.Dispatch('WbemScripting.SWbemLocator')
      self.handler = self.server.ConnectServer(host, namespace)
    except self._pywintypes.com_error as e:
      raise WmiError('Failure connecting to namespace: %s' % str(e))

  def Query(self, query):
    """Run WMI Query on a machine.

    Args:
      query: (string) WQL query you want to run.

    Returns:
        (list) returns the query result as a list.
    """
    try:
      return self.handler.ExecQuery(query)
    except self._pywintypes.com_error as e:
      raise WmiError('Failure executing query: %s' % str(e))

  def _WmiInit(self):
    """Initialize the pythoncom and win32com modules.

    Raises:
      WmiError: failure to initialize a pythoncom or win32com module instance.
    """
    try:
      import pythoncom  # pylint: disable=g-import-not-at-top
      self._pythoncom = pythoncom
      self._pythoncom.CoInitialize()
    except ImportError:
      raise WmiError('No pythoncom module available on this platform.')

    try:
      from win32com import client  # pylint: disable=g-import-not-at-top
      self._client = client
    except ImportError:
      raise WmiError('No win32com module available on this platform.')

    try:
      import pywintypes  # pylint: disable=g-import-not-at-top
      self._pywintypes = pywintypes
    except ImportError:
      raise WmiError('No pywintypes module available on this platform.')
