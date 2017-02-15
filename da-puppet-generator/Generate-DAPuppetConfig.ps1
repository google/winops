# Copyright 2017 Google Inc.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

<#
  .SYNOPSIS
    Convert DirectAccess Group Policy Objects to Puppet Config.

  .DESCRIPTION
    Using search strings, find DirectAccess GPOs then recursively grab all
    registry settings applied by that GPO. Convert those registry settings to
    puppet code while de-duplicating settings and renaming named collisions.
#>

param (
  [parameter(Position = 0, Mandatory = $false, ValueFromPipeline = $true)]
      $config_file
)

# Variables
$DNSCLIENT_STRING = 'HKLM\Software\Policies\Microsoft\Windows NT\DNSClient\DnsPolicyConfig\DA-{*}'
# Setting defaults
$GPO_BASE = '<Default Search String>'
$GPO_DCA_BASE = '<Default Search String>'
$DA_GROUP_SUFFIX = '<Default Search String>'
$OUTFILE_BASE = 'C:\path\to\folder'
$DA_SERVICES = @('mpssvc', 'dnscache', 'iphlpsvc')
# Excluding settings that are known to cause problems.
$EXCLUDED_SETTINGS = @()
$EXCLUDED_SETTINGS += 'RegValue1'
$EXCLUDED_SETTINGS += 'RegValue2'
# Settings to exclude from 'All' due to conflicts.
$EXCLUDE_FROM_ALL = @()
# Setting applied with different value in Global vs Regional.
$EXCLUDE_FROM_ALL += 'RegValue3'
$EXCLUDE_FROM_ALL += 'RegValue4'

# Read config file if specified.
if ($config_file) {
  Write-Host "Reading options from $config_file"
  [xml]$configs = Get-Content $config_file

  if ($configs.Settings.GPO_BASE) {
    $GPO_BASE = $configs.Settings.GPO_BASE
  }

  if ($configs.Settings.GPO_DCA_BASE) {
    $GPO_DCA_BASE = $configs.Settings.GPO_DCA_BASE
  }

  if ($configs.Settings.DA_GROUP_SUFFIX) {
    $DA_GROUP_SUFFIX= $configs.Settings.DA_GROUP_SUFFIX
  }

  if ($configs.Settings.OUTFILE_BASE) {
    $OUTFILE_BASE = $configs.Settings.OUTFILE_BASE
  }

  if ($configs.Settings.DA_SERVICES) {
    $DA_SERVICES = @()
    foreach ($value in $configs.Settings.DA_SERVICES.Value) {
      $DA_SERVICES += $value
    }
  }

  if ($configs.Settings.EXCLUDED_SETTINGS) {
    $EXCLUDED_SETTINGS = @()
    foreach ($value in $configs.Settings.EXCLUDED_SETTINGS.Value) {
      $EXCLUDED_SETTINGS += $value
    }
  }

  if ($configs.Settings.EXCLUDE_FROM_ALL) {
    $EXCLUDE_FROM_ALL = @()
    foreach ($value in $configs.Settings.EXCLUDE_FROM_ALL.Value) {
      $EXCLUDE_FROM_ALL += $value
    }
  }
}


function Check-DuplicateSetting {
  <#
    .SYNOPSIS
      Check for duplicate setting and set to All.

    .PARAMETER setting
      Array of setting to be applied.

    .PARAMETER puppet_conf
      Puppet configuration working set.
  #>
  param (
    [parameter(Position = 0, Mandatory = $true, ValueFromPipeline = $true)]
      $setting,
    [parameter(Position = 1, Mandatory = $true, ValueFromPipeline = $true)]
      $puppet_conf
  )
  if ($setting.ValueName -notin $EXCLUDE_FROM_ALL) {
    foreach ($value in $puppet_conf) {
      if ($value.ValueName -eq $setting.ValueName -and
          $value.FullKeyPath -eq $setting.FullKeyPath -and
          $value.Type -eq $setting.Type -and
          $value.Value -eq $setting.Value) {
        $value.EntryPoint = 'All'
        return $puppet_conf
      }
    }
  }
  $puppet_conf = [Array]$puppet_conf + $setting
  return $puppet_conf
}

function Process-RegistryKeys {
  <#
    .SYNOPSIS
      Recursively traverse subkeys and built actual puppet config.

    .PARAMETER guid
      GUID of GPO.

    .PARAMETER path
      Registry path to the keys we are processing.

    .PARAMETER puppet_conf
      Puppet configuration working set.
  #>
  param (
    [parameter(Position = 0, Mandatory = $true, ValueFromPipeline = $true)]
      [String]$guid,
    [parameter(Position = 1, Mandatory = $true, ValueFromPipeline = $true)]
      [String]$path,
    [parameter(Position = 2, Mandatory = $true, ValueFromPipeline = $true)]
      $puppet_conf
  )
  foreach ($key in (Get-GPRegistryValue -GUID $guid -Key $path `
      -ErrorAction SilentlyContinue)) {
    if ($key.HasValue) {
      if ($key.ValueName -notin $EXCLUDED_SETTINGS) {
        if ($key.FullKeyPath -like '*WindowsFirewall*') {
          $config = 'firewall'
        }
        else {
          $config = 'client_networking'
        }

        if ($key.Type -eq 'MultiString') {
          $type = 'array'
        }
        else {
          $type = ([string]$key.Type).ToLower()
        }

        # String replacements to play nice with Puppet standards.
        $value_name = $key.ValueName -Replace('\\','/')
        $full_key_path = $key.FullKeyPath -Replace('HKEY_LOCAL_MACHINE','HKLM')
        $full_key_path = $full_key_path -Replace('\\Software\\','\SOFTWARE\')
        $full_key_path = $full_key_path -Replace('\\System\\','\SYSTEM\')

        $properties = @{
            EntryPoint=$gpo.DisplayName.Split('-')[1].Trim();
            PuppetName=$value_name;
            ValueName=$value_name;
            FullKeyPath=$full_key_path;
            Type=$type;
            Value=([string]$key.Value).Trim([char]0);
            Config=$config
        }
        $reg_key = New-Object PSObject -Property $properties

        $puppet_conf = Check-DuplicateSetting $reg_key $puppet_conf
      }
    }
    else {
      $sub_path = $key.FullKeyPath
      $puppet_conf = Process-RegistryKeys $guid $sub_path $puppet_conf
    }
  }
  if ($puppet_conf) {
    return $puppet_conf
  }
  # If GPO has no values in it this function will return false. (In the case of
  # a staged, but not configured GPO.)
  return $false
}


function Get-PuppetFromGPO {
  <#
    .SYNOPSIS
      Build puppet-formatted config from Registry files stored in a GPO.

    .PARAMETER gpo_base_name
      Search String to use for processing multiple GPOs or targeting a single
      GPO by name.

    .PARAMETER registry_base
      Starting point in the registry to start recursively searching.
  #>
  param (
    [parameter(Position = 0, Mandatory = $true, ValueFromPipeline = $true)]
      [String]$gpo_base_name,
    [parameter(Position = 1, Mandatory = $true, ValueFromPipeline = $true)]
      [String]$registry_base
  )
  $puppet_conf = @()

  foreach ($gpo in (Get-GPO -All | Where-Object { $_.DisplayName `
      -like $gpo_base_name })) {
    $group_name = ($gpo | Get-GPPermissions -All |
        Where-Object {$_.Permission -eq 'GpoApply'}).Trustee.Name
    $guid = $gpo.Path.Split('=,')[1]

    Write-Host ('Reading ' + $gpo.DisplayName)

    $return = Process-RegistryKeys $guid $registry_base $puppet_conf

    if ($return) {
      $puppet_conf = $return
    }
  }
  return $puppet_conf
}

function Remove-NameCollisions {
  <#
    .SYNOPSIS
      Write puppet configs to output files.

    .PARAMETER puppet_conf
      Puppet configuration working set.
  #>
  param (
    [parameter(Position = 0, Mandatory = $true, ValueFromPipeline = $true)]
      $puppet_conf
  )
  $seen_values = @()

  foreach ($setting in $puppet_conf) {
    if ($seen_values -contains $setting.PuppetName) {
      # Collision detected, set new PuppetName.
      # Some firewall rules are resistant to de-duplication.
      if ($setting.FullKeyPath.Split('\')[-1] -match '^[0-9][0-9][0-9][0-9]$') {
        $partial_key = ($setting.FullKeyPath.Split('\')[-2] + '\' + `
            $setting.FullKeyPath.Split('\')[-1])
      }
      elseif ($setting.FullKeyPath.Split('\')[-1] -eq 'IPHTTPSInterface') {
        $partial_key = ($setting.FullKeyPath.Split('\')[-2] + '\' + `
            $setting.FullKeyPath.Split('\')[-1])
      }
      else {
        $partial_key = $setting.FullKeyPath.Split('\')[-1]
      }

      $setting.PuppetName = ($setting.ValueName + ' - ' + `
          $partial_key + ' - ' + $setting.EntryPoint)
    }
    else {
      $seen_values += $setting.PuppetName
    }
  }
  return $puppet_conf
}

function Find-DnsClientKeys {
  <#
    .SYNOPSIS
      Identify policy keys associated with DNSClient. Having more than one set
      of these keys applied will break DA on the client.

    .PARAMETER puppet_conf
      Puppet configuration working set.
  #>
  param (
    [parameter(Position = 0, Mandatory = $true, ValueFromPipeline = $true)]
      $puppet_conf
  )
  $reg_extract = 'DA-{(.{8}-.{4}-.{4}-.{17})}'
  $dnsclient_keys = @()

  foreach ($setting in $puppet_conf) {
    if ($setting.FullKeyPath -like $DNSCLIENT_STRING) {
      if ($setting.ValueName -eq 'Name') {
        $guid = [regex]::match($setting.FullKeyPath, $reg_extract).Groups[1].Value
        $properties = @{
            Guid=$guid;
            EntryPoint=$setting.EntryPoint;
            Value=$setting.Value;
        }
        $dns_key = New-Object PSObject -Property $properties
        $dnsclient_keys += $dns_key
      }
    }
  }
  return $dnsclient_keys
}

function Write-PuppetConfigs {
  <#
    .SYNOPSIS
      Write puppet configs to output files.

    .PARAMETER config_type
      Determines which config set to use.

    .PARAMETER out_file_base
      File base to calculate file to write to.

    .PARAMETER puppet_conf
      Puppet configuration working set.

    .PARAMETER dnsclient_keys
      Object containing a list of DNS Client key settings.
  #>
  param (
    [parameter(Position = 0, Mandatory = $true, ValueFromPipeline = $true)]
      [String]$config_type,
    [parameter(Position = 1, Mandatory = $true, ValueFromPipeline = $true)]
      [String]$out_file_base,
    [parameter(Position = 2, Mandatory = $true, ValueFromPipeline = $true)]
      $puppet_conf,
    [parameter(Position = 3, Mandatory = $false, ValueFromPipeline = $true)]
      $dnsclient_keys = $null
  )
  # Set $out_file.
  $out_file = "$out_file_base\directaccess-$config_type.pp"

  Write-Host "Writing file $out_file."

  # Build list of entry points.
  $entry_points = @()
  foreach ($setting in $puppet_conf) {
    if (!($entry_points -contains $setting.EntryPoint)) {
      $entry_points += $setting.EntryPoint
    }
  }

  # Write Network Config
  $null > $out_file
  # Write headers
  "class $config_type::directaccess {" >> $out_file

  # Write out services.
  if ($config_type -eq 'client_networking') {
    foreach ($service in $DA_SERVICES) {
      '' >> $out_file
      "  service { '$service':" >> $out_file
      "    ensure => 'running'," >> $out_file
      '    enable => true;' >> $out_file
      '  }' >> $out_file
    }
  }

  if ($dnsclient_keys) {
    # Write DNS_KEY Hashtable
    '' >> $out_file
    '  $dns_guids = [' >> $out_file

    foreach ($key in $dnsclient_keys) {
      "      {'guid' => '" + $key.Guid + "', 'ep' => '" + $key.EntryPoint + "', 'name' => '" + $key.Value + "'}," >> $out_file
    }

    '  ]' >> $out_file
    '' >> $out_file
    '  windows::da_dnsclient_key { $dns_guids: }' >> $out_file
    '' >> $out_file
  }

  # Loop through entry points.
  foreach ($ep in ($entry_points | Sort-Object)) {
    if ($ep) {
      Write-Host "Writing $ep."
      # Write EP conditional. (With logic around All & Global)
      if ($ep -eq 'All') {
        "  if `$da_group =~ /$DA_GROUP_SUFFIX/ {" >> $out_file
        '' >> $out_file
      }
      elseif ($ep -eq 'Global') {
        "  if `$da_group == '$DA_GROUP_SUFFIX' {" >> $out_file
      }
      else {
        "  if `$da_group == '$ep-$DA_GROUP_SUFFIX' {" >> $out_file
      }

      # Write registry::value opening
      '    registry::value {' >> $out_file
      "      # DirectAccess Config for Entry Point '$ep'" >> $out_file

      # Loop through settings for Network & EP
      foreach ($setting in $puppet_conf) {
        if ($setting.FullKeyPath -notlike $DNSCLIENT_STRING) {
          if ($setting.Config -eq $config_type -and $setting.EntryPoint -eq $ep) {
            "        '" + $setting.PuppetName + "':" >> $out_file
            if ($setting.PuppetName -ne $setting.ValueName) {
            "          value => '" + $setting.ValueName + "'," >> $out_file
            }
            "          key => '" + $setting.FullKeyPath + "'," >> $out_file
            '          type => ' + $setting.Type + ',' >> $out_file
            "          data => '" + $setting.Value + "';" >> $out_file
            '' >> $out_file
          }
        }
      }

      if ($config_type -eq 'client_networking' -and !($ep -eq 'All')) {
        # Write da_entry_point to registry
        "        'da_entry_point':" >> $out_file
        "          key => 'HKLM\SOFTWARE\Policies\Microsoft\Windows\RemoteAccess\Config'," >> $out_file
        '          type => string,' >> $out_file
        "          data => '$ep'," >> $out_file
        "          notify => Service['" + ($DA_SERVICES -join "', '") + "'];" >> $out_file
      }

      # Write entry point closing before possible else statement
      '    }' >> $out_file
      '  }' >> $out_file

      # Space between entry points
      '' >> $out_file
    }
  }

  # Write closing

  # Write footers
  '}' >> $out_file
}

# Main
$start_time = Get-Date
$puppet_conf = @()

# Grab Actual DA Configs
Write-Host 'Collecting DirectAccess Configs'
$puppet_conf += Get-PuppetFromGPO $GPO_BASE 'HKEY_LOCAL_MACHINE\Software'
$puppet_conf += Get-PuppetFromGPO $GPO_BASE 'HKEY_LOCAL_MACHINE\System'

# Grab DCA Configs
Write-Host 'Collecting DCA Configs'
$puppet_conf += Get-PuppetFromGPO $GPO_DCA_BASE 'HKEY_LOCAL_MACHINE\Software'

# Remove name collisions
Write-Host 'Cleaning Name Collisions'
$puppet_conf = Remove-NameCollisions $puppet_conf

# Identify DNSClient Policy Settings. This is so unused keys can be removed from
# the registry to avoid breaking DirectAccess.
Write-Host 'Identifying DNSClient Keys'
$dnsclient_keys = Find-DnsClientKeys $puppet_conf

Write-Host 'Writing configs to file'
Write-PuppetConfigs 'client_networking' $OUTFILE_BASE $puppet_conf `
    $dnsclient_keys
Write-PuppetConfigs 'firewall' $OUTFILE_BASE $puppet_conf

$end_time = Get-Date

Write-Host ('Finished in ' + ($end_time - $start_time).TotalMinutes + `
    ' minutes.')
