# rdbsh

`rdbsh` is a RocksDB shell for inspecting local databases interactively or by running a single command.

Use it when you want to:

- look up individual keys
- scan keys or key-value pairs by prefix
- count entries in a database or column family
- inspect RocksDB properties and table statistics
- export all data or a filtered subset to CSV or JSON

`rdbsh` opens databases in read-only mode by default.

## Install

Install the latest release:

```sh
./scripts/install.sh rdbsh
```

Install without cloning the repo:

```sh
curl -fsSL https://raw.githubusercontent.com/jo-cube/toolbox/main/scripts/install.sh | sh -s -- rdbsh
```

Check the installed version:

```sh
rdbsh --version
```

## Quick Start

Open a database interactively:

```sh
rdbsh --db /tmp/offsetstorage
```

Run a single command and exit:

```sh
rdbsh --db /tmp/offsetstorage --exec "count"
```

Target a specific column family:

```sh
rdbsh --db /tmp/offsetstorage --cf offsets --exec "scan 0x00 20"
```

Enable writes explicitly:

```sh
rdbsh --db /tmp/offsetstorage --writable
```

## Command Reference

```sh
rdbsh --db <path> [options]
```

Required flags:

- `--db`: path to the RocksDB data directory

Optional flags:

- `--writable`: open the database in read-write mode
- `--cf`: column family to operate on; defaults to the default column family
- `--exec`: run a single shell command and exit
- `--version`: print version information

Shell commands:

- `get <key>`: read a single key
- `put <key> <value>`: write a key-value pair; requires `--writable`
- `delete <key>`: delete a key; requires `--writable`
- `scan [prefix] [limit]`: list keys and values; default limit is `100`
- `keys [prefix] [limit]`: list keys only; default limit is `100`
- `count [prefix]`: count keys, optionally by prefix
- `stats`: print RocksDB properties for the selected column family
- `export <file|-> [csv|json] [prefix]`: export entries to a file or stdout
- `cfs`: list available column families and mark the selected one
- `help`: show shell help
- `exit` or `quit`: leave the interactive shell

## Input Format

All key, value, and prefix arguments accept either plain text or raw bytes encoded as hex.

Examples:

- `get mykey`
- `get 0x00000001`
- `put hello world`
- `put 0x01 0xdeadbeef`
- `scan 0x0000 10`
- `count 0x00`

Rules:

- Prefix with `0x` or `0X` to decode hexadecimal bytes
- Without `0x`, the input is treated as literal UTF-8 text
- Use double quotes for arguments containing spaces
- Use `\` to escape spaces or quotes in the shell

Printable bytes are shown as text. Non-printable bytes are shown as lowercase hex.

## Common Workflows

Read a single key:

```sh
rdbsh --db /tmp/offsetstorage --exec "get 0x00000001"
```

List keys with a prefix:

```sh
rdbsh --db /tmp/offsetstorage --exec "keys 0x00 50"
```

Scan a column family interactively:

```sh
rdbsh --db /tmp/offsetstorage --cf offsets
```

Export the full database to JSON:

```sh
rdbsh --db /tmp/offsetstorage --exec "export /tmp/dump.json json"
```

Pipe filtered JSON to another command:

```sh
rdbsh --db /tmp/offsetstorage --exec "export - json 0x00"
```

## Export Formats

CSV export writes a header row:

```text
key,value
```

JSON export writes an array of objects in this shape:

```json
{
  "key": "00000001",
  "value": "0000000000209163"
}
```

When the export target is `-`, data is written to stdout and the completion message is written to stderr.

## Column Families

Use `--cf <name>` to operate on a non-default column family.

Use `cfs` inside the shell to discover available column families:

```text
* default
  offsets
  metadata
```

The selected column family is marked with `*`.

## Safety Notes

- `rdbsh` is read-only unless you pass `--writable`
- Use `--exec` for repeatable one-shot reads in scripts
- Be careful with `put` and `delete` on production data stores
- Large scans and exports may take time on very large databases

## Build From Source

`rdbsh` uses CGo and links against RocksDB.

macOS:

```sh
brew install rocksdb
make build
```

Ubuntu or Debian:

```sh
sudo apt-get update
sudo apt-get install -y librocksdb-dev
make build
```

The `Makefile` tries `pkg-config` first and falls back to common Homebrew locations.

## Troubleshooting

The database does not open:

- Confirm `--db` points to a RocksDB directory, not a file
- Check that the process has permission to read the directory

The column family is missing:

- Run without `--cf` and use `cfs` to list available names
- Confirm the exact column family name and casing

The build cannot find `rocksdb/c.h`:

- Install RocksDB development headers for your platform
- Confirm the headers and libraries are visible via `pkg-config` or your system include paths
