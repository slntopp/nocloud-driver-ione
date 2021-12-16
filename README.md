# nocloud-driver-ione: IONe Driver for NoCloud

## Service Config

See `examples/templates/service.yml` for an example service template you can use with nocloud CLI

or `examples/requests/service.yml` for an example HTTP request body you can use with Postman, cURL

## Services Provider Config

See `examples/templates/sp.yml` for an example services provider template you can use with nocloud CLI

or `examples/requests/sp.yml` for an example HTTP request body you can use with Postman, cURL

## Invoke IONe Method

Request schema is defined as:

```proto
message Request {
    google.protobuf.Value action = 1;
    repeated google.protobuf.Value params = 2;
}
```

Actions stands for method path as if you'd do requests to IONe REST-API. So resulting requests schemas are:

```jsonc
// IONe Method
{
    "action": "ione/Method",
    "params": [
        arg0, arg1
    ]
}
// OpenNebula API Method
{
    "action": "one.obj.method",
    "params": [
        object_id, arg0, arg1
    ]
}
```
