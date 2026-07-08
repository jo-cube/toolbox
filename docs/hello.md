# hello

`hello` prints a small greeting.

It is also the minimal reference CLI in `toolbox`: use it to understand the expected package split, version flag, help text, and exit behavior before adding a new simple tool.

## Install

Install the latest release:

```sh
curl -fsSL https://raw.githubusercontent.com/jo-cube/toolbox/main/scripts/install.sh | sh -s -- hello
```

Install from a cloned repo:

```sh
./scripts/install.sh hello
```

Check the installed version:

```sh
hello --version
```

## Synopsis

```sh
hello
hello --version
hello -h
```

## Examples

Print the greeting:

```sh
hello
```

Output:

```text
Hello, world!
```

Print version information:

```sh
hello --version
```

## Options

- `--version`: print version information

## Output

`hello` writes the greeting to stdout.

Errors, if any, are written to stderr.

## Exit Status

- `0`: success
- `1`: runtime error
- `2`: invalid command-line usage

## Contributor Notes

- CLI flags and process exit behavior live in `cmd/hello/main.go`.
- The testable greeting behavior lives in `internal/hello`.
- Build-time version output comes from `internal/buildinfo`.
- Keep this tool boring. It is useful because it is the smallest complete CLI in the repo.
