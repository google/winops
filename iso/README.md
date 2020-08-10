# Overview

The iso package provides access to raw ISO images for Go binaries running on
Windows, Linux and Mac OS. Implementations are kept identical between platforms
to ensure testability through interfaces by consumers. This package supports
raw ISO files (\*.iso).

## Features

* **Support for Mounting and Dismounting** - A consumer running the Mount
function will obtain an object representing the mounted ISO. The ISO can be
dismounted at will, and can cleanup after itself.

* **Automatic Inspection** - A mounted ISO is inspected to learn where the
original file is located, the location of the mounted filesystem, the contents
of the ISO and its overall size.

* **Copying** - The contents of a mounted ISO can be copied to a different
location.

* **Consistent implementations** - An ISO is accessed using the same methods,
regardless of platform. In other words, tasks like searching for a drive are
accomplished the same way on Windows, Linux or Mac.

* **Built for Testing** - Consumers of this library can create interfaces in
their applications to mock the ISO, permitting consumers to test
expected behaviors.

## Requirements

This library utilizes native OS tooling to achieve a uniform result. See the
following list for the OS specific requirements.

* **Windows** - Windows 10 with Powershell 5.0 or higher. Powershell is
leveraged to perform ISO handling functions.

* **Linux** - GNU Linux with the mount command available.

* **Mac** - MacOS Mojave or higher with access to the mount command.

## How to use this library

A few example uses for this library are illustrated below. For additional use
cases, see the codebase.

### Mounting an ISO

Mount an ISO on the filesystem located at path.

```
path := `C:\Temp\test.iso`
handler, err := iso.New(path)
if err != nil {
  // Log Error message here.
}
```

### Inspecting ISO contents

After mounting an ISO, obtain a list of its contents in real-time and print
the contents of the ISO.

```
handler, err := iso.New(path)
if err != nil {
  // Log Error message here.
}
fmt.Printf("Contents of ISO at %q: %v", handler.ImagePath(), handler.Contents())

```

### Copying ISO contents

Copy the contents of a mounted iso (handler) to a directory on the local drive.

```
handler, err := iso.New(path)
if err != nil {
  // Log error message here.
}

dest := "/tmp/isocopy"
if err := handler.Copy(dest); err != nil {
  // Log error message here.
}

```

## Contact

We have a public discussion list at
[google-winops@googlegroups.com](https://groups.google.com/forum/#!forum/google-winops)

## Disclaimer

This is not an official Google product.


