# Overview

This is a script that will convert DirectAccess 2012 configuration into puppet
manifests. It was written with the purpose of automating the complex process of
updating thousands of lines of puppet code whenever DirectAccess configuration
changes as well as to remove the reliance on DirectAccess to be able to update
and repair it's own configuration.

## Features

The following settings are configurable either directly in the script itself or
can be passed via an xml config file.

-   `GPO_BASE`: Base search string to identify DirectAccess GPOs.
-   `GPO_DCA_BASE`: Base search string to identify DirectAccess Connectivity
    Assistant GPOs.
-   `DA_GROUP_SUFFIX`: Base search string to identify DirectAccess Active
    Directory Groups.
-   `OUTFILE_BASE`: Folder in which to write the puppet config files.
-   `DA_SERVICES`: Client services to restart should any changes to
    configuration be applied.
-   `EXCLUDED_SETTINGS`: Registry Values to exclude from adding to puppet
    manifests.
-   `EXCLUDE_FROM_ALL`: Registry Values to exclude from de-duplication process.

The script will output two files. One with network-related settings and the
other with firewall-related settings. These files are designed to be included in
a `client_networking` and `firewall` module within your manifests.

## Requirements

The generated client_networking manifest will leverage the
da_dnsclient_key addon module. This should be placed in your addons folder and
included appropriately.

The puppet manifests rely on having the client's DirectAccess Active Directory
Group in a facter fact named `da_group`.

The script is written with the assumption that any DirectAccesss Connectivity
Assistant settings are stored in a separate GPO. This can be skipped by
commenting out the 2 lines after `# Grab DCA Configs`.

The script works on the assumption that your DirectAccess clients are sorted
into Active Directory Groups with the following naming convention:

If `DA_Clients` is defined as your `DA_GROUPS_SUFFIX`

-   `???-DA_Clients` for Windows 7 Clients
-   `DA_Clients` for Windows 8+ Clients

## Contact

We have a public discussion list at
[google-winops@googlegroups.com](https://groups.google.com/forum/#!forum/google-winops)

## Disclaimer

This is not an official Google product.
