# Overview

`gwinpy` is a set of python modules with useful methods for managing and Windows
computers.

## Features

All modules and methods should have helpful docstrings. Some of the highlights:

-   `gwinpy.wmi` has methods to extract data from host WMI
    -   `gwinpy.wmi.wmi_query` is the generic wmi query interface which can be
        extended for additional queries.

## Requirements

`gwinpy.wmi` requires the `pythoncom`, `pywintypes`, and `win32com` python
modules.

Unit tests also require the [mock][] module.

## Tests

Most of the code is covered with unit tests. These can be run by executing: `$
python -m unittest discover -p '*_test.py'

[mock]: http://www.voidspace.org.uk/python/mock/

## Contact

We have a public discussion list at
[google-winops@googlegroups.com](https://groups.google.com/forum/#!forum/google-winops)

## Disclaimer

This is not an official Google product.
