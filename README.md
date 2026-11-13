# GoFlake

Goflake is a disributed uuid generator. By default it uses SnowFlake configuration.

Goflake comes with both `grpc` and `http` servers so you can use it in your own applications without any additional dependencies (including TLS).

See [announcing snowflake](https://blog.twitter.com/engineering/en_us/a/2010/announcing-snowflake) by Twitter.

## Install

```bash
go install github.com/Avielyo10/goflake@latest
```

## Configuration

Goflake will try to read configuration from `/etc/goflake/config.yaml` or `$CWD/config.yaml` if it exists, else default will apply. You can also specify configuration from environment variables to override configuration from file.

For example:

```bash
env "SERVER.TYPE=rest" DATACENTER_ID=1 MACHINE_ID=2 go run .
```

### Configuration File in detail

* `log_level`: either debug, info, warn, error, fatal
* `env`: either development, production
* `datacenter_id`: can be 0 - 2 ^ flake.bits_len.datacenter_id - 1
* `machine_id`: can be 0 - 2 ^ flake.bits_len.machine_id - 1

* `flake configuration`
  * `bits_len`: bits length of flake id, needs to sum to 63 (1 bit for sign)
    * `datacenter_id`: uint64, default 5
    * `machine_id`: uint64, default 5
    * `sequence`: uint64, default 12
    * `time`: uint64, default 41
  * `epoch`: epoch of flake id in unix milliseconds, must not be in the future, default is Thu Jul 28 2022 18:57:35 UTC
  * `tick_ms`: length of one time unit in milliseconds, the sequence restarts from 0 every tick, default is 1ms

* `server configuration`
  * `host`: server host
  * `port`: server port
  * `tls configuration`
    * `cert_path`: path to certificate file
    * `key_path`: path to key file
  * `type`: either grpc, rest

See example [config](./examples/config.yaml)

Goflake refuses to start with an invalid configuration, for example a `datacenter_id` or `machine_id` that does not fit in its bits, as two nodes sharing the same ids generate the same uuids.

## API

| | rest | grpc |
|---|---|---|
| new uuid | `GET /v1/uuid` | `goflake.FlakeService/GetUUID` |
| decompose a uuid | `GET /v1/decompose/:uuid` | `goflake.FlakeService/Decompose` |

A decomposed uuid reports when it was generated as `timestamp` (RFC 3339), the rest server also returns the raw `time` (ticks since the epoch).

Generating a uuid fails (`503` / `UNAVAILABLE`) rather than return a duplicate when the clock is before the epoch, moved backwards by more than goflake can wait out, or the time no longer fits in `flake.bits_len.time` bits.

Goflake has no authentication, run it on a trusted network or behind a proxy that takes care of it. The docker image runs as an unprivileged user (`65534`), files mounted into it (configuration, tls certificate and key) need to be readable by that user.
