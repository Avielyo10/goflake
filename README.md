# GoFlake

Goflake is a disributed uuid generator. By default it uses SnowFlake configuration.

Goflake comes with both `grpc` and `http` servers so you can use it in your own applications without any additional dependencies (including TLS).

See [announcing snowflake](https://blog.twitter.com/engineering/en_us/a/2010/announcing-snowflake) by Twitter.

## Install

```bash
go install github.com/Avielyo10/goflake
```

## Configuration

Goflake will try to read configuration from `$HOME/.goflake/config.yaml` or `$PWD/config.yaml` if it exists, else default will apply. You can also specify configuration from environment variables to override configuration from file.

For example:
```bash
env "SERVER.TYPE=rest" DATACENTER_ID=1 MACHINE_ID=2 go run ./internal/
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
  * `epoch`: epoch of flake id, default is Thu Jul 28 2022 18:57:35 UTC
  * `tick_ms`: tick interval in milliseconds to set sequence to 0, default is 1ms

* `server configuration`
  * `host`: server host
  * `port`: server port
  * `tls configuration`
    * `cert_path`: path to certificate file
    * `enabled`: false
    * `key_path`: path to key file
  * `type`: either grpc, rest

See example [config](./config-example.yaml)
