# ksetoff

`ksetoff` sets Kafka consumer group offsets for a topic without starting the consumer application.

Use it when you want the next consumer in a group to start from a specific point, such as:

- the earliest available data
- the latest offset
- an exact numeric offset
- the first offset at or after a timestamp

## Install

Install the latest release:

```sh
./scripts/install.sh ksetoff
```

Install without cloning the repo:

```sh
curl -fsSL https://raw.githubusercontent.com/jo-cube/toolbox/main/scripts/install.sh | sh -s -- ksetoff
```

Check the installed version:

```sh
ksetoff --version
```

## Quick Start

1. Create or locate a Kafka config file in kcat or librdkafka `key=value` format.
2. Run `ksetoff` in `-dry-run` mode first.
3. Review the plan and any warnings.
4. Re-run without `-dry-run` to commit offsets.

Example dry run:

```sh
ksetoff \
  -F ~/.config/kcat/my-cluster.conf \
  -group my-consumer-group \
  -topic my-topic \
  -offset latest \
  -dry-run
```

Commit the change:

```sh
ksetoff \
  -F ~/.config/kcat/my-cluster.conf \
  -group my-consumer-group \
  -topic my-topic \
  -offset latest
```

## Common Workflows

Set a group to the earliest available offset:

```sh
ksetoff -F kafka.conf -group my-cg -topic my-topic -offset earliest
```

Set a group to the latest offset:

```sh
ksetoff -F kafka.conf -group my-cg -topic my-topic -offset latest
```

Set a group to a specific numeric offset:

```sh
ksetoff -F kafka.conf -group my-cg -topic my-topic -offset 42
```

Set offsets based on a timestamp:

```sh
ksetoff -F kafka.conf -group my-cg -topic my-topic -offset "timestamp:2026-04-13T00:00:00Z"
```

Target specific partitions only:

```sh
ksetoff -F kafka.conf -group my-cg -topic my-topic -offset 100 -partitions 0,1,2
```

Preview the change without committing:

```sh
ksetoff -F kafka.conf -group my-cg -topic my-topic -offset latest -dry-run
```

## Command Reference

```sh
ksetoff -F <config> -group <group-id> -topic <topic> -offset <spec> [options]
```

Required flags:

- `-F`: path to a kcat-style config file
- `-group`: consumer group ID
- `-topic`: Kafka topic
- `-offset`: target offset spec

Optional flags:

- `-partitions`: comma-separated partition numbers; defaults to all partitions
- `-dry-run`: print the plan without committing offsets
- `-timeout`: overall operation timeout; defaults to `30s`
- `--version`: print version information

Supported offset specs:

- `<number>`: exact numeric offset
- `earliest`: beginning of each target partition
- `latest`: end of each target partition
- `timestamp:<ISO-8601>`: first offset at or after a timestamp

Accepted aliases:

- `beginning`: same as `earliest`
- `end`: same as `latest`

Accepted timestamp examples:

- `timestamp:2026-04-13T00:00:00Z`
- `timestamp:2026-04-13T00:00:00`
- `timestamp:2026-04-13 00:00:00`
- `timestamp:2026-04-13`

## Kafka Config File

`ksetoff` reads connection settings from a kcat or librdkafka-style `key=value` file.

Supported keys:

- `bootstrap.servers=broker1:9092,broker2:9092`
- `metadata.broker.list=broker1:9092,broker2:9092`
- `security.protocol=PLAINTEXT|SSL|SASL_PLAINTEXT|SASL_SSL`
- `sasl.mechanism=PLAIN|SCRAM-SHA-256|SCRAM-SHA-512`
- `sasl.username=my-user`
- `sasl.password=my-password`
- `ssl.ca.location=/path/to/ca.pem`
- `ssl.certificate.location=/path/to/client.pem`
- `ssl.key.location=/path/to/client.key`
- `ssl.key.password=secret`
- `enable.ssl.certificate.verification=true`

Example config:

```text
bootstrap.servers=broker1:9092,broker2:9092
security.protocol=SASL_SSL
sasl.mechanism=SCRAM-SHA-512
sasl.username=my-user
sasl.password=my-password
ssl.ca.location=/path/to/ca.pem
```

Notes:

- One of `bootstrap.servers` or `metadata.broker.list` is required.
- Unknown keys are ignored.
- If you use mTLS, both `ssl.certificate.location` and `ssl.key.location` must be set.
- Encrypted private keys are not currently supported. If `ssl.key.password` is set, `ksetoff` returns a clear error.

## What Output To Expect

`ksetoff` prints a plan showing:

- target group
- topic
- requested offset mode
- per-partition current offset
- per-partition new offset
- low and high watermarks

Warnings are printed when a requested offset is outside the available range.

- If the requested offset is below the low watermark, the consumer will start from the earliest available data.
- If the requested offset is above the high watermark, the consumer will wait for new messages.

## Safety Notes

- Run with `-dry-run` first.
- Stop active consumers in the target group before committing offsets.
- Confirm you are pointing at the correct cluster and topic before applying changes.

## Troubleshooting

The group has active members:

- Stop all consumers in the target group.
- Retry the command after the group becomes inactive.

Connection or authentication fails:

- Check `bootstrap.servers`.
- Check `security.protocol` and SASL settings.
- Check CA and client certificate paths.

The offsets look unexpected:

- Re-run with `-dry-run`.
- Verify the timestamp and timezone used in `timestamp:<ISO-8601>`.
- Compare the planned offsets against the topic watermarks shown in the output.
