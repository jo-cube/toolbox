# rdbsh

`rdbsh` is a RocksDB shell for inspecting local databases interactively or with a single command.

Use it when you want to:

- look up individual keys
- scan keys or key-value pairs by prefix
- count entries in a database or column family
- inspect RocksDB properties and table statistics
- export all data or a filtered subset to CSV or JSON

`rdbsh` opens databases in read-only mode unless you pass `--writable`.
It opens the database locally; it does not connect to a remote service.

## Install

Install the latest release:

```sh
curl -fsSL https://raw.githubusercontent.com/jo-cube/toolbox/main/scripts/install.sh | sh -s -- rdbsh
```

The released binary requires a compatible RocksDB runtime library. The installer checks this before installing and prints platform-specific setup guidance when the library is unavailable.

Install from a cloned repo:

```sh
./scripts/install.sh rdbsh
```

Check the installed version:

```sh
rdbsh --version
```

## Synopsis

```sh
rdbsh --db <path> [options]
```

## Examples

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

Read a single key:

```sh
rdbsh --db /tmp/offsetstorage --exec "get 0x00000001"
```

List keys with a prefix:

```sh
rdbsh --db /tmp/offsetstorage --exec "keys 0x00 50"
```

Export the full database to JSON:

```sh
rdbsh --db /tmp/offsetstorage --exec "export /tmp/dump.json json"
```

Pipe filtered JSON to another command:

```sh
rdbsh --db /tmp/offsetstorage --exec "export - json 0x00"
```

## Options

Required flags:

- `--db`: path to the RocksDB data directory

Optional flags:

- `--writable`: open the database in read-write mode
- `--cf`: column family to operate on; defaults to the default column family
- `--exec`: run a single shell command and exit
- `--force`: allow `export <file>` to overwrite an existing file
- `--version`, `-V`: print version information

## Shell Commands

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

Write commands:

- `put` and `delete` require `--writable`.
- `export` does not require `--writable`; it reads data and writes the export target.
- `export <file>` fails if the file already exists unless `--force` is set.

## Input Format

Keys, values, and prefixes can be plain text or raw bytes encoded as hex.

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

Printable bytes are shown as text. Non-printable bytes are shown as `0x`-prefixed lowercase hex. Empty values are shown as `(empty)`.

## Export Formats

CSV export writes a header row:

```text
key,value
```

JSON export writes an array of objects in this shape:

```json
[
  {
    "key": "0x00000001",
    "value": "0x0000000000209163"
  }
]
```

When the export target is `-`, data is written to stdout and the completion message is written to stderr.

Exporting to a file fails if the file already exists unless `--force` is set.

Exported keys and values are round-trippable through the shell input format. Ordinary printable text stays readable. Empty, binary, and text beginning with `0x` or `0X` is emitted as unambiguous lowercase hex; for example, an empty value is `0x`, a zero byte is `0x00`, and the literal text `0x00` is `0x30783030`.

## Output

Human-readable command output is written to stdout.

Diagnostics and command errors are written to stderr.

Automation behavior:

- `--exec` runs one shell command and exits.
- In interactive mode, command errors are printed to stderr and the shell keeps running.
- In `export - ...`, exported data is written to stdout and the completion message is written to stderr.
- Large `scan`, `count`, and `export` commands can be slow on large databases because they iterate keys.

## Exit Status

- `0`: success
- `1`: runtime error
- `2`: invalid command-line usage

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

The installer reports that the released binary is missing a RocksDB library:

- On macOS, run `brew install rocksdb`
- On Ubuntu or Debian, run `sudo apt-get install librocksdb-dev`
- On other Linux distributions, install a RocksDB runtime compatible with the release binary

The column family is missing:

- Run without `--cf` and use `cfs` to list available names
- Confirm the exact column family name and casing

The build cannot find `rocksdb/c.h`:

- Install RocksDB development headers for your platform
- Confirm the headers and libraries are visible via `pkg-config` or your system include paths

## Contributor Notes

- CLI flags and process exit behavior live in `cmd/rdbsh/main.go`.
- Shell commands, parsing, formatting, iteration, and export behavior live in `internal/rdbsh`.
- The CGo surface is intentionally narrow and lives in `internal/rdbsh/rocksdb`.
- Keep read-only mode as the default. Writes should remain explicit through `--writable`.
