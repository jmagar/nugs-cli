# Nugs CLI Command Mapping (2026-02-05)

## Quick Reference

### ◆ LIST COMMANDS

─────────────────────────────────────────────────────────────────────────────

- `list artists`  List all available artists
  - `nugs list`
- `list artists shows ">100"`  Filter artists by show count (`>`, `<`, `>=`, `<=`, `=`)
  - `nugs list ">100"`
- `list <artist_id>`  List all shows for a specific artist
  - `nugs list 1125`
- `list <artist_id> shows "venue"`  Filter shows by venue name
  - `nugs list 1125 "Red Rocks"`
- `list <artist_id> latest <N>`  Show latest N shows for an artist
  - `nugs list 1125 latest 5`
- `<artist_id> latest`  Download latest shows from an artist
  - `nugs grab 1125 latest`

### ◆ CATALOG COMMANDS

─────────────────────────────────────────────────────────────────────────────

- `catalog update`  Fetch and cache latest catalog
  - `nugs update`
- `catalog cache`  Show cache status and metadata
  - `nugs cache`
- `catalog stats`  Display catalog statistics
  - `nugs stats`
- `catalog latest [limit]`  Show latest additions (default 15)
  - `nugs latest 10`
- `catalog gaps <id> [...]`  Find missing shows (one or more artists)
  - `nugs gaps 1125 461`
- `catalog gaps <id> --ids-only`  Output IDs for piping
  - `nugs gaps 1125 --ids-only`
- `catalog gaps <id> fill`  Auto-download all missing shows
  - `nugs gaps 1125 fill`
- `catalog coverage [ids...]`  Show download coverage statistics
  - `nugs coverage 1125 461`
- `catalog config enable|disable|set`  Configure auto-refresh
  - `nugs refresh enable`

## Migration Notes (Old -> New)

- `nugs list artists` -> `nugs list`
- `nugs list artists shows ">100"` -> `nugs list ">100"`
- `nugs list 461 shows "Red Rocks"` -> `nugs list 461 "Red Rocks"`
- `nugs 12345` -> `nugs grab 12345`
- `nugs 461 latest` -> `nugs grab 461 latest`
- `nugs catalog update` -> `nugs update`
- `nugs catalog gaps 1125` -> `nugs gaps 1125`
- `nugs catalog gaps 1125 fill` -> `nugs gaps 1125 fill`
- `nugs catalog coverage 1125 461` -> `nugs coverage 1125 461`
- `nugs help` -> `nugs help` (unchanged)
