# Overview

The Storage package provides uniform access to the underlying storage layer for
Windows, Linux and Mac OS. Implementations are kept identical between platforms
to ensure testability through interfaces by consumers. This package supports
Physical and Removable Hard Disk storage. Other storage types are not available.

## Features

* **Device and Partition Support** - Storage devices returned by this library
are described with both device and partitioning attributes. Consumers can use
the various accessor methods to inspect the contents of a device and evaluate it
for use.

* **Support for Formatting and Partitioning** - Storage devices can be wiped,
partitioned and formatted as needed.

* **Support for Mounting/Dismounting/Ejecting** - Storage devices can be,
mounted, dismounted or ejected.

* **Consistent implementations** - Storage layers are accessed using the same
methods, regardless of platform. In other words, tasks like searching for a
drive are accomplished the same way on Windows, Linux or Mac.

* **Built for Testing** - Consumers of this library can create interfaces in
their applications to mock the storage layer, permitting consumers to test
expected storage behaviors.

## Requirements

Storage utilizes native OS implementations to achieve a uniform result. See the
following list for the OS specific requirements.

* **Windows** - Windows 10 with Powershell 5.0 or higher. Powershell is
leveraged to perform many disk management functions.

* **Linux** - GNU Linux with the parted and label commands available. It is
assumed that the user can sudo to root for specific actions.

* **Mac** - MacOS Mojave or higher with access to the diskutil command.

## How to use this library

A few example uses for this library are illustrated below. For additional use
cases, see the codebase.

### Search for storage devices

This call returns one or more devices if it is successful. Available parameters
include an optional deviceID (when searching for a specific device), a minimum
and maximum size and and whether or not to return only removable devices.

```
deviceID := ""          // An empty string searches all devices.
minSize := 1000000000   // 1 GB
maxSize := 100000000000 // 100 GB
removableOnly := true   // Return only removable drives.

// Search removable devices for storage devices that are > 1GB, but < 100GB.
devices, err := storage.Search(deviceID, minSize, maxSize, removableOnly)
if err != nil {
  // Log Error message here.
}
```

### Retrieve a storage device

This call retrieves information about a device and returns an object that you
can use to inspect or modify the device.

```
deviceID := "sda"
device, err := storage.New(deviceID)
if err != nil {
  // Log error message here.
}

```

### Repartition a storage device

Use these calls to repartition a device already previously retrieved and assign
a label of "NEWLBL".

```
if err := device.Wipe(); err != nil {
  // Log error message here.
}

label := "NEWLBL"
if err := device.Partition(label); err != nil {
  // Log error message here.
}
```

## Contact

We have a public discussion list at
[google-winops@googlegroups.com](https://groups.google.com/forum/#!forum/google-winops)

## Disclaimer

This is not an official Google product.


