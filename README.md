# nocloud-driver-ione: IONe Driver for NoCloud

## Service Config

See `examples/templates/service.yml` for an example service template you can use with nocloud CLI

or `examples/requests/service.yml` for an example HTTP request body you can use with Postman, cURL

## Services Provider Config

See `examples/templates/sp.yml` for an example services provider template you can use with nocloud CLI

or `examples/requests/sp.yml` for an example HTTP request body you can use with Postman, cURL

## Setup Hook

### Get binary

```sh
# Get link from Releases page
wget https://github.com/slntopp/nocloud-driver-ione/releases/download/v0.0.0-r1/nocloud-ione-v0.0.0-r1-linux-amd64.tar.gz
# Unpack
tar -xvf nocloud-ione-v0.0.0-r1-linux-amd64.tar.gz
# Move binary to OpenNebula hooks dir (optional)
mv nocloud-ione ~oneadmin/remotes/hooks
```

### Configure

1. Create `/etc/one/ione.yaml`
2. Fill in host and insecure (and vnc/vmrc data to add VNC support)

    ```yaml
    host: api.your.nocloud:8080
    insecure: false

    SUNSTONE_VNC_TOKENS_DIR: /var/lib/one/sunstone_vnc_tokens
    SUNSTONE_VMRC_TOKENS_DIR: /var/lib/one/sunstone_vmrc_tokens/
    SOCKET_VMRC_ENDPOINT: ws://localhost/fireedge/vmrc/
    SOCKET_VNC_ENDPOINT: ws://localhost:29876
    ```

3. Run `nocloud-ione test`. Result must be `true`.
4. Run `nocloud-ione hooks`

### Uninstall

1. Run `nocloud-ione hooks cleanup`
2. Delete binary

## Setup VNC

### Install & Configure

`nocloud-ione-vnc` gives an API endpoint to generate VNC tokens consumable by Driver VNC Proxy and Tunnel.

1. Get the Archive from Releases page
2. Unpack it
3. Run `sh install.sh` (`install.sh` is included into Archive)
4. Fill `/etc/one/ione.yaml`

Configure nginx. This API must be available as `<ONe RPC2 endpoint>/vnc`(for example `https://one.example.com/RPC2/vnc`).

You could achieve this by adding:

```nginx
    location /RPC2/vnc {
        proxy_pass              http://127.0.0.1:8010;
        proxy_http_version      1.1;
        proxy_set_header        X-Real-IP               $remote_addr;
        proxy_set_header        X-Forwarded-For         $proxy_add_x_forwarded_for;
        proxy_set_header        X-Forwarded-Server      $host;
        proxy_set_header        host                    $host;
        proxy_set_header        Upgrade                 $http_upgrade;
        proxy_set_header        Connection              $connection_upgrade;
        add_header              Access-Control-Allow-Origin *;
    }
```

### Uninstall

1. Stop daemon `systemctl stop nocloud-ione-vnc`
2. Delete `/usr/lib/systemd/system/nocloud-ione-vnc.service`
3. Delete binary (`/usr/bin/nocloud-ione-vnc`)
