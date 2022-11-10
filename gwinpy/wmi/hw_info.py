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
"""Classes to enumerate basic hardware data from WMI."""

import logging
import re
from gwinpy.wmi import wmi_query


class PnpEntity(object):
  """Store PnpEntity objects corresponding to local devices.

  Note: This can be extended to store additional properties as needed.
  """

  def __init__(self, caption=None, device_id=None):
    self.caption = caption
    self.device_id = device_id


class DeviceId(object):
  """Store hardware Device IDs."""

  def __init__(self, ven=None, dev=None, subsys=None, rev=None):
    self.ven = ven
    self.dev = dev
    self.subsys = subsys
    self.rev = rev

  def __Stringify(self):
    """Represent the current object as a device id text string."""
    dev_str = ''
    if self.ven:
      dev_str = '%s' % self.ven
      if self.dev:
        dev_str += '-%s' % self.dev
        if self.subsys:
          dev_str += '-%s' % self.subsys
          if self.rev:
            dev_str += '-%s' % self.rev
    return dev_str

  def __str__(self):
    return self.__Stringify()

  def __repr__(self):
    return self.__Stringify()


class HWInfo(object):
  """Query basic hardware data in WMI."""

  def __init__(self):
    self.wmi = wmi_query.WMIQuery()

  def BiosSerial(self):
    """Get the BIOS serial from Win32_BIOS.

    Returns:
      The SerialNumber string if found; else None.
    """
    query = 'Select SerialNumber from Win32_BIOS'
    results = self.wmi.Query(query)
    if results:
      logging.debug('Win32_BIOS/SerialNumber: %s', results[0].SerialNumber)
      return results[0].SerialNumber
    logging.warning('No results for %s.', query)
    return None

  def BIOSVersion(self):
    """Get the BIOS version from Win32_BIOS.

    Returns:
      The Version string if found; else None.
    """
    query = 'Select SMBIOSBIOSVersion from Win32_BIOS'
    results = self.wmi.Query(query)
    if results:
      logging.debug('Win32_BIOS/SMBIOSBIOSVersion: %s',
                    results[0].SMBIOSBIOSVersion)
      return results[0].SMBIOSBIOSVersion
    logging.warning('No results for %s.', query)
    return None

  def ChassisType(self):
    """Get the system chassis type from Win32_SystemEnclosure.

    Returns:
      The first chassis type found; else None.
    """
    query = 'Select ChassisTypes from Win32_SystemEnclosure'
    results = self.wmi.Query(query)
    if results:
      for chassisconfig in results:
        for chassis in chassisconfig.chassistypes:
          logging.debug('Win32_SystemEnclosure/ChassisType: %s', chassis)
          return chassis
    logging.warning('No results for %s.', query)
    return None

  def ComputerSystemManufacturer(self):
    """Get the system manufacturer from Win32_ComputerSystem.

    Returns:
      The Manufacturer string if found; else None.
    """
    query = 'Select Manufacturer from Win32_ComputerSystem'
    results = self.wmi.Query(query)
    if results:
      logging.debug('Win32_ComputerSystem/Manufacturer: %s',
                    results[0].Manufacturer.rstrip())
      return results[0].Manufacturer.rstrip()
    logging.warning('No results for %s.', query)
    return None

  def ComputerSystemModel(self):
    """Get the system model from Win32_ComputerSystem.

    Returns:
      The Model string if found; else None.
    """
    query = 'Select Model from Win32_ComputerSystem'
    results = self.wmi.Query(query)
    if results:
      logging.debug('Win32_ComputerSystem/Model: %s', results[0].Model.rstrip())
      return results[0].Model.rstrip()
    logging.warning('No results for %s.', query)
    return None

  def HDDSerial(self):
    """Get the HDD serial from Win32_PhysicalMedia.

    Returns:
      The SerialNumber string if found; else None.
    """
    query = ('SELECT SerialNumber from Win32_PhysicalMedia '
             'WHERE Tag LIKE "%DRIVE0"')
    results = self.wmi.Query(query)
    if results:
      logging.debug('Win32_PhysicalMedia/SerialNumber: %s',
                    results[0].SerialNumber.strip())
      return results[0].SerialNumber.strip()
    logging.warning('No results for %s.', query)
    return None

  def IsLaptop(self):
    """Detect whether the local machine appears to be a laptop.

    Returns:
      true for laptops; else false
    """
    if self.ChassisType() in [8, 9, 10, 11, 12, 14, 30, 31, 32]:
      return True
    return False

  def IsVirtualMachine(self):
    """Detect whether the local machine appears to be virtual hardware.

    Returns:
      true for virtual machines; else false
    """
    model = self.ComputerSystemModel().lower()
    if 'virtual' in model:
      logging.debug('Detected generic virtual machine.')
      return True
    elif 'vmware' in model:
      logging.debug('Detected VMWare virtual machine.')
      return True
    elif 'parallels' in model:
      logging.debug('Detected Parallels virtual machine.')
      return True
    elif 'Google Compute Engine' in model:
      logging.debug('Detected Google Compute Engine virtual machine.')
      return True
    logging.debug('No virtual hardware detected.')
    return False

  def IsOnBattery(self):
    """Detect whether the local machine appears to be on battery.

    Returns:
      true if on battery; else false
    """
    query = 'SELECT BatteryStatus FROM Win32_Battery'
    unplugged_statuses = [1, 13, 14, 15, 16, 17]  # on battery or battery saver
    results = self.wmi.Query(query)
    if results:
      for result in results:
        logging.debug('Found Win32_Battery with BatteryStatus: %s',
                      result.BatteryStatus)
        if result.BatteryStatus in unplugged_statuses:
          return True
    else:
      logging.debug('No Win32_Battery detected.')
    return False

  def LenovoSystemModel(self):
    """Get the Lenovo-specific common model name (instead of the model number).

    Returns:
      The model string if found; else None.
    """
    query = 'SELECT Version FROM Win32_ComputerSystemProduct'
    results = self.wmi.Query(query)
    if results:
      logging.debug('Win32_ComputerSystemProduct/Version: %s',
                    results[0].Version.rstrip())
      return results[0].Version.rstrip()
    logging.warning('No results for %s.', query)
    return None

  def MacAddresses(self, pci_only=False):
    """Get the physical host mac addresses from Win32_NetworkAdapter.

    Args:
      pci_only: limit search to PNPDeviceID LIKE "PCI%"

    Returns:
      A list of all mac addresses found.
    """
    query = ('SELECT MacAddress FROM Win32_NetworkAdapter WHERE '
             'PhysicalAdapter=1 AND AdapterTypeID=0')
    if pci_only:
      query += ' AND PNPDeviceID LIKE "PCI%"'
    results = self.wmi.Query(query)
    addresses = []
    for adapter in results:
      address = adapter.MacAddress
      logging.debug('Win32_NetworkAdapter/MacAddress: %s', address)
      addresses.append(address)
    return addresses

  def PciDevices(self):
    """Get local PCI devices.

    Returns:
      A list of DeviceId objects containing the ven/dev/subsys/rev strings.
    """
    devices = []
    query = ('Select * From Win32_PnpEntity where DeviceID like "%SUBSYS%"')
    results = self.wmi.Query(query)
    if results:
      pci_device = re.compile(
          r'^PCI\\VEN_(\w+)&DEV_(\w+)&SUBSYS_(\w+)&REV_(\w+)\\')
      for result in results:
        match = pci_device.match(result.DeviceID)
        if match:
          devices.append(
              DeviceId(
                  ven=match.group(1),
                  dev=match.group(2),
                  subsys=match.group(3),
                  rev=match.group(4)))
    else:
      logging.warning('No results for %s.', query)
    return devices

  def PnpDevices(self, device_id=None):
    """Get local Plug and Play devices.

    Args:
      device_id: Retrieve a specific device by its device id.

    Returns:
      A list of PnpEntity objects for each local device.
    """
    devices = []
    query = ('Select * from Win32_PnPEntity')
    if device_id:
      query += (' where DeviceID="%s"' % device_id)
    results = self.wmi.Query(query)
    if results:
      for result in results:
        try:
          devices.append(PnpEntity(caption=result.Caption))
        except AttributeError:
          logging.warning('No Caption for device. [%s]', str(result))
    return devices

  def SmbiosUuid(self):
    """Gets the SMBIOS UUID from Win32_ComputerSystemProduct.

    Returns:
      The UUID string if found; else None.
    """
    query = ('Select UUID from Win32_ComputerSystemProduct')
    results = self.wmi.Query(query)
    if results:
      logging.debug('Win32_ComputerSystemProduct/UUID: %s',
                    results[0].UUID.strip())
      return results[0].UUID.strip()
    logging.warning('No results for %s.', query)
    return None

  def UsbDevices(self):
    """Get local USB devices.

    Returns:
      A list of PnpEntity objects for each local USB device.
    """
    devices = []
    query = ('Select * from Win32_USBControllerDevice')
    results = self.wmi.Query(query)
    if results:
      for usb_device in results:
        try:
          device_dependent = usb_device.Dependent
        except AttributeError:
          logging.warning('No dependent for USB device %s.', usb_device)
          continue
        logging.debug('Found dependent USB device %s.', device_dependent)
        device_id = re.search('.*="(.*)"', device_dependent).groups(0)[0]
        if device_id:
          pnp_dev = self.PnpDevices(device_id=device_id)
          if pnp_dev:
            devices.append(pnp_dev[0])
    else:
      logging.warning('No results for %s.', query)
    return devices

  def VideoControllers(self):
    """Get all video controllers (graphics cards) present in the system.

    Returns:
      A list with one dict element per controller detected.  Each dict contains
      values describing the controller.
        description: controller description (str)
        driver_version: version of the driver (str)
        name: controller name (str)
        pnpid: the PNP device id (str)
        ram_size: the RAM present in the adapter (int)
    """
    controllers = []
    query = 'SELECT * FROM Win32_VideoController'
    results = self.wmi.Query(query)
    if results:
      for controller in results:
        logging.debug('Win32_VideoController: %s', controller)
        controllers.append({
            'description': controller.Description.rstrip(),
            'driver_version': controller.DriverVersion.rstrip(),
            'name': controller.Name.rstrip(),
            'pnpid': controller.PNPDeviceID.rstrip(),
            'ram_size': controller.AdapterRAM,
        })
    else:
      logging.warning('No results for %s.', query)
    return controllers
