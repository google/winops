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
"""Basic Windows registry handling."""

from typing import List, Optional, Any, Tuple

from six.moves import builtins

if 'WindowsError' in builtins.__dict__:
  WindowsError = WindowsError  # pylint: disable=undefined-variable, disable=self-assigning-variable
else:

  class WindowsError(Exception):
    pass


class RegistryError(Exception):  # pylint: disable=g-bad-exception-name
  """Class defining default/required fields when throwing a RegistryError."""

  def __init__(self, message='', errno=0):
    self.errno = errno
    super(RegistryError, self).__init__(message)


class Registry(object):
  """Class providing basic interaction with the Windows registry."""

  def __init__(self, root_key='HKLM'):
    self._WinRegInit()
    if root_key not in self._root_map:
      raise RegistryError('Failed to open unsupported root key: %s' % root_key)
    self._root_key = root_key
    self._root_key_value = self._root_map[root_key]

  def GetKeyValue(self,
                  key_path: str,
                  key_name: str,
                  use_64bit: Optional[bool] = True) -> str:
    r"""function to retrieve a Windows registry value.

    Args:
      key_path: the key we'll search in (such as SOFTWARE\Microsoft)
      key_name: the key that we want to get our value from (such as
        ProgramFilesDir)
      use_64bit: use the 64bit registry rather than 32bit

    Returns:
      the value of the specified registry key.

      Raises:
        RegistryError: failure opening a handle to the requested key_path
    """
    try:
      handle = self._OpenSubKey(key_path, create=False, use_64bit=use_64bit)
      result = self._winreg.QueryValueEx(handle, key_name)[0]
      handle.Close()
      return result
    except WindowsError as e:
      raise RegistryError(
          r'Failed to get registry value: %s:\%s\%s (%s)' %
          (self._root_key, key_path, key_name, e),
          errno=e.errno) from e

  def GetRegKeys(self,
                 key_path: str,
                 use_64bit: Optional[bool] = True) -> List[str]:
    r"""function to enumerate through a subkey and return key names.

    Args:
      key_path: the key we'll search (such as SOFTWARE\Microsoft)
      use_64bit: use the 64bit registry rather than 32bit

    Returns:
      A list of registry keys.

    Raises:
      RegistryError: failure opening a handle to the requested key_path
    """
    results = []
    try:
      handle = self._OpenSubKey(key_path, create=False, use_64bit=use_64bit)
      # https://docs.microsoft.com/en-us/windows/win32/sysinfo/registry-element-size-limits
      for subkeys in range(512):
        result = self._winreg.EnumKey(handle, subkeys)
        results.append(result)
    except OSError as e:
      # WindowsError: [Errno 259] No more data is available
      if e.winerror == 259:
        return results
      raise RegistryError(
          r'Failed to open registry key: %s:\%s (%s)' %
          (self._root_key, key_path, e),
          errno=e.errno) from e
    finally:
      handle.Close()

  def GetRegKeysAndValues(
      self,
      key_path: str,
      use_64bit: Optional[bool] = True) -> List[Tuple[str, Any, int]]:
    r"""function to enumerate through a subkey and return key names and values.

    Args:
      key_path: the key we'll search (such as SOFTWARE\Microsoft)
      use_64bit: use the 64bit registry rather than 32bit

    Returns:
      A list containing tuples of registry keys:
        [(data, name, type), (data, name, type), ...]

    Raises:
      RegistryError: failure opening a handle to the requested key_path
    """
    results = []
    try:
      handle = self._OpenSubKey(key_path, create=False, use_64bit=use_64bit)
      key = self._winreg.OpenKey(self._root_key_value, key_path, 0,
                                 self._winreg.KEY_READ)
      # https://docs.microsoft.com/en-us/windows/win32/sysinfo/registry-element-size-limits
      for subkeys in range(512):
        results.append(self._winreg.EnumValue(key, subkeys))
    except OSError as e:
      # WindowsError: [Errno 259] No more data is available
      if e.winerror == 259:
        return results
      raise RegistryError(
          r'Failed to open registry key: %s:\%s (%s)' %
          (self._root_key, key_path, e),
          errno=e.errno) from e
    finally:
      handle.Close()

  def _OpenSubKey(self,
                  key_path: str,
                  create: Optional[bool] = True,
                  write: Optional[bool] = False,
                  use_64bit: Optional[bool] = True) -> Any:
    """Connect to the local registry key.

    Args:
      key_path: the registry subkey to be opened
      create: whether to create the key_path if not found
      write: open the key for write instead of read
      use_64bit: use the 64bit registry rather than 32bit

    Returns:
      A registry handle to the specified key_path.

    Raises:
      RegistryError: failure opening a handle to the requested key_path
    """
    registry_view = 0
    if use_64bit:
      registry_view = 256

    access = self._winreg.KEY_READ
    if write:
      access = self._winreg.KEY_WRITE

    try:
      if create:
        return self._winreg.CreateKeyEx(self._root_key_value, key_path, 0,
                                        access
                                        | registry_view)
      return self._winreg.OpenKey(self._root_key_value, key_path, 0, access
                                  | registry_view)
    except WindowsError as e:
      raise RegistryError(
          r'Failed to open registry key: %s:\%s (%s)' %
          (self._root_key, key_path, e),
          errno=e.errno) from e

  def SetKeyValue(self,
                  key_path: str,
                  key_name: str,
                  key_value: str,
                  key_type: Optional[str] = 'REG_SZ',
                  use_64bit: Optional[bool] = True) -> None:
    r"""function to retrieve a Windows registry value.

    Args:
      key_path: the key we'll search in (such as Software\Microsoft)
      key_name: the key that we want to get our value from (such as
        ProgramFilesDir)
      key_value: the new desired value for key_name
      key_type: a supported key type (eg REG_SZ, REG_DWORD)
      use_64bit: use the 64bit registry rather than 32bit

    Raises:
      RegistryError: failure opening a handle to the requested key_path
    """
    if key_type not in self._type_map:
      raise RegistryError('Attempt to create key of invalid type. [%s]' %
                          key_type)

    try:
      handle = self._OpenSubKey(
          key_path, create=True, write=True, use_64bit=use_64bit)
      self._winreg.SetValueEx(handle, key_name, 0, self._type_map[key_type],
                              key_value)
    except WindowsError as e:
      raise RegistryError(
          r'Failed to read registry value: %s:\%s\%s\%s (%s)' %
          (self._root_key, key_path, key_name, key_value, e),
          errno=e.errno) from e
    finally:
      handle.Close()

  def RemoveKeyValue(self,
                     key_path: str,
                     key_name: str,
                     use_64bit: Optional[bool] = True) -> None:
    r"""function to remove a Windows registry value.

    Args:
      key_path: the key we'll search in (such as Software\Microsoft)
      key_name: the key that we want to get our value from (such as
        ProgramFilesDir)
      use_64bit: use the 64bit registry rather than 32bit

    Raises:
      RegistryError: failure opening a handle to the requested key_path
    """

    try:
      handle = self._OpenSubKey(
          key_path, create=False, write=True, use_64bit=use_64bit)
      self._winreg.DeleteValue(handle, key_name)
    except WindowsError as e:
      raise RegistryError(
          r'Failed to delete registry key: %s:\%s\%s (%s)' %
          (self._root_key, key_path, key_name, e),
          errno=e.errno) from e
    finally:
      handle.Close()

  def _WinRegInit(self):
    """Initialize the _winreg module and dependent variables.

    Raises:
      RegistryError: failure to initialize a _winreg module instance
    """
    try:
      from six.moves import winreg  # pylint: disable=g-import-not-at-top
      self._winreg = winreg
    except ImportError:
      raise RegistryError('No winreg module available on this platform.')
    self._root_map = {
        'HKCR': self._winreg.HKEY_CLASSES_ROOT,
        'HKCU': self._winreg.HKEY_CURRENT_USER,
        'HKLM': self._winreg.HKEY_LOCAL_MACHINE,
        'HKU': self._winreg.HKEY_USERS,
    }
    self._type_map = {
        'REG_DWORD': self._winreg.REG_DWORD,
        'REG_SZ': self._winreg.REG_SZ,
    }
