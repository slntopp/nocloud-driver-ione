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
2. Fill in host and insecure

    ```yaml
    host: api.your.nocloud:8080
    insecure: false
    ```

3. Run `nocloud-ione test`. Result must be `true`.
4. Run `nocloud-ione hooks`

### Uninstall

1. Run `nocloud-ione hooks cleanup`
2. Delete binary
