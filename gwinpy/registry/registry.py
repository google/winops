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

from six.moves import builtins

if 'WindowsError' in builtins.__dict__:
  # pylint: disable=used-before-assignment
  # pylint: disable=g-bad-name
  WindowsError = WindowsError  # pylint: disable=undefined-variable
else:

  class WindowsError(Exception):
    pass


class RegistryError(Exception):
  """Class defining default/required fields when throwing a RegistryError."""

  def __init__(self, message='', errno=0):
    self.errno = errno
    super(RegistryError, self).__init__(message)


class Registry(object):
  """Class providing basic interaction with the Windows registry."""

  def __init__(self, root_key='HKLM'):
    self._WinRegInit()
    if root_key not in self._root_map:
      raise RegistryError('Attempting to open unsupported root key. [%s]' %
                          root_key)
    self._root_key = self._root_map[root_key]

  def GetKeyValue(self, key_path, key_name, use_64bit=True):
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
      raise RegistryError('Failed to read %s from %s.\n%s' %
                          (key_name, key_path, e))

  def _OpenSubKey(self, key_path, create=True, write=False, use_64bit=True):
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
        return self._winreg.CreateKeyEx(self._root_key, key_path, 0, access
                                        | registry_view)
      return self._winreg.OpenKey(self._root_key, key_path, 0, access
                                  | registry_view)
    except WindowsError as e:
      raise RegistryError(
          'Failure opening requested key. [%s]\n%s' % (key_path, e),
          errno=e.errno)

  def SetKeyValue(self,
                  key_path,
                  key_name,
                  key_value,
                  key_type='REG_SZ',
                  use_64bit=True):
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
      handle.Close()
    except WindowsError as e:
      raise RegistryError('Failed to read %s from %s.\n%s' %
                          (key_name, key_path, e))

  def RemoveKeyValue(self,
                     key_path,
                     key_name,
                     use_64bit=True):
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
      handle.Close()
    except WindowsError as e:
      raise RegistryError(
          'Failed to delete %s from %s.\n%s' % (key_name, key_path, e),
          errno=e.errno)

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
