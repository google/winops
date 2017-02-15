define windows::da_dnsclient_key($dns_key = $title) {
  $key = sprintf('HKEY_LOCAL_MACHINE\Software\Policies\Microsoft\Windows NT\DNSClient\DnsPolicyConfig\DA-{%s}', $dns_key['guid'])

  if $dns_key['ep'] != 'All' and $client_base::da_entrypoint != sprintf('%s', $dns_key['ep']) {
    registry_key {
      # Removing DNSClient config for other entry points.
      "Remove ${key}":
        ensure => absent,
        path => $key,
        purge_values => true;
    }
  }
  else {
    if $dns_key['name'] == '<PUT NLS URL HERE>' {
      $dns_server = ''
      $da_proxy_type = '1'
    }
    else {
      $dns_server = '<PUT DNS64 ADDRESS HERE>'
      $da_proxy_type = '0'
    }

    registry::value {
      "Version - ${dns_key}['guid']":
        value => 'Version',
        key => $key,
        type => dword,
        data => '1';

      "ConfigOptions - ${dns_key}['guid']":
        value => 'ConfigOptions',
        key => $key,
        type => dword,
        data => '4';

      "Name - ${dns_key}['guid']":
        value => 'Name',
        key => $key,
        type => array,
        data => $dns_key['name'];

      "DirectAccessDNSServers - ${dns_key}['guid']":
        value => 'DirectAccessDNSServers',
        key => $key,
        type => string,
        data => $dns_server;

      "DirectAccessProxyName - ${dns_key}['guid']":
        value => 'DirectAccessProxyName',
        key => $key,
        type => string,
        data => '';

      "DirectAccessProxyType - ${dns_key}['guid']":
        value => 'DirectAccessProxyType',
        key => $key,
        type => dword,
        data => $da_proxy_type;

      "DirectAccessQueryIPSECEncryption - ${dns_key}['guid']":
        value => 'DirectAccessQueryIPSECEncryption',
        key => $key,
        type => dword,
        data => '0';

      "DirectAccessQueryIPSECRequired - ${dns_key}['guid']":
        value => 'DirectAccessQueryIPSECRequired',
        key => $key,
        type => dword,
        data => '0';

      "IPSECCARestriction - ${dns_key}['guid']":
        value => 'IPSECCARestriction',
        key => $key,
        type => string,
        data => '';
    }
  }
}
