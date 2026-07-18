# Nugs CLI Quick Reference

## Setup

```bash
make build
nugs
chmod 600 ~/.nugs/config.json
```

## Download

```bash
nugs grab 23329
nugs grab 23329 video
nugs 1125 latest audio
nugs 1125 full
```

## Browse and catalog

```bash
nugs list
nugs list 1125 video
nugs list 1125 latest 5
nugs update
nugs stats
nugs latest 50
nugs gaps 1125 audio
nugs coverage
```

`nugs latest` cannot filter by media. With no IDs, `nugs coverage` examines
artists found in local and configured remote download folders.

## Watch

```bash
nugs watch add 1125
nugs watch list
nugs watch check
nugs watch enable
nugs watch disable
```

## Interactive controls

- `Shift+P`: pause or resume
- `Shift+C`: cancel
- `Ctrl+C`: interrupt

## Verify

```bash
make test
make verify
```

See [COMMANDS.md](COMMANDS.md) and [CONFIG.md](CONFIG.md) for complete contracts.
