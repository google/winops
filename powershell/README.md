# Overview

The powershell package provides a standard go-compatible method for invoking
powershell on Windows. It handles errors returned by the shell and by default
also handles errors thrown by cmdlets and returns them to the caller. The
powershell configuration is configurable.
## Features

* **Simple calling, handling and defaults** - Running powershell from your go
binary is as simple as making a single function call to Command. Calls to
command require no additional input outside of the psCmd you wish to run. Errors
from powershell are automatically handled and returned.


* **Modifiable Powershell Configuration** - If a configuration other than the
default is desired, the user can provide an alternative PSConfig when making
powershell calls. This allows for calls to specify alternate errorActions and
additional parameters.


* **Customizable error handling** - The caller can specify alternate text that
indicates an error as a parameter, this provides the ability to throw an error
if the output of the powershell command contains output that the caller
considers an error, regardless of whether an error was returned by powershell.

* **Simpler Testing** - Consumers of this library can use dependency injection
to mock the powershell calls, confident that handling of the powershell call
itself is consistent. They can then write tests that do not need to test
that powershell is run successfully, only that its output or errors are handled.

## Requirements

* **Windows** - Windows 10 or Windows Server 2016 with Powershell 5.0 or higher.

* **Linux** - This library is not supported on Linux.

## How to use this library

A few example uses for this library are illustrated below. This list is not
exhaustive, see the codebase for additional information.

### Run a powershell command with the default configuration.

This call runs a single powershell command (contained on a single line) and
returns the output to the caller.

```
psCmd := `Get-Process Chrome | Select ProcessName | ConvertTo-Json`

out, err := powershell.Command(psCmd, nil, nil)
if err != nil {
  // Log Error message here.
}
// Process out here.
```

### Run a powershell command with an alternate configuration.

This call runs a single powershell command (contained on a single line), using
an alternate configuration (PSConfig).

```
psCmd := `Get-Process Chrome | Select ProcessName | ConvertTo-Json`
cfg := &powershell.PSConfig{ErrAction: powershell.SilentlyContinue}

out, err := powershell.Command(psCmd, nil, cfg)
if err != nil {
  // Log Error message here.
}
// Process out here.
```

### Run a powershell command and throw an error when specific output is found.

This call runs a single powershell command (contained on a single line), and
specifies an error should be thrown if the output contains 'chrome'. This is to
say that if a process named 'chrome' is found running, an error should be
returned.

```
psCmd := `Get-Process Chrome | Select ProcessName | ConvertTo-Json`
suppErr := []string{"chrome"}

out, err := powershell.Command(psCmd, suppErr, nil)
if err != nil {
  // Do something now that you've found Chrome in the output.
}
// Process out here.
```

## Contact

We have a public discussion list at
[google-winops@googlegroups.com](https://groups.google.com/forum/#!forum/google-winops)

## Disclaimer

This is not an official Google product.

