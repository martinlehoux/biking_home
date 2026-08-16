# biking_home — cycling ride analysis

A Go toolkit for analyzing cycling rides from GPX exports: parse rides, detect climbs, score their difficulty, match the same climb across rides, and download French mountain passes into SQLite.

## Features

- **GPX parsing** — reads elevation-tracked rides from Garmin/Strava GPX exports into a `Ride` model
- **Climb detection** — automatically finds climbs within a ride, each scored and categorized (Cat 4 … HC)
- **Difficulty score (Cotacol)** — scores any ride or climb by splitting it into 100 m segments and summing `0.1 km × slope²` per segment. This fixed step is a deliberate compromise: smaller steps increase sensitivity to elevation noise, while unconstrained variable steps maximize noise rather than meaningful difficulty.
- **Similar-climb matching** — finds the same climb across rides by matching start/end coordinates, so times can be compared
- **Mountain pass download** — imports French mountain passes from centcols.org into a SQLite database
- **Pass crossing detection** — enriches passes with OSM coordinates, flags which passes a ride crosses, and names each climb after the pass it tops (e.g. "Col de Castellaras")
- **Interactive ride profiles** — explores elevation, climbs, and mountain-pass crossings in the web ride detail view
- **Official climb cards** — separates matched climbs from pending matches and expands each matched climb with its own elevation profile
- **Web ride library** — starts a local web server by default, imports Strava rides over a date range, stores metadata in SQLite, and keeps their GPX files on disk
- **Materialized ride values** — computes Cotacol on import, stores its algorithm version in SQLite, and refreshes all computed values with `-backfill`
- **Strava stream metrics** — preserves heart rate, cadence, and power in Garmin-compatible GPX files and in-memory ride columns
- **Metadata-only ride imports** — stores Strava activity statistics for rides without GPS tracks, without Cotacol or route data
- **Ride table sorting** — sorts every ride-library column through clickable server-side headers and query parameters
- **Ride detail maps** — opens a stored ride with its recorded route on an interactive OpenStreetMap map
- **Strava import progress** — streams live import progress with an aggregate progress bar and prevents overlapping syncs

## Getting started

```bash
go build -o biking_home .
```

## Usage

```bash
# Start the web server on http://localhost:8080
mise watch dev-watch --restart --exts go,templ,css,js --ignore '*_templ.go'

# Recompute and persist all ride computed values
./biking_home -backfill

# Download French mountain passes into biking_home.db (SQLite)
./biking_home -download

# Resume an interrupted download, reusing departments already cached on disk
./biking_home -download -resume

# Import cached department CSVs (debug_department_06.csv, 13.csv) into the DB
./biking_home -import-cached

# Extract mountain passes from a France OSM PBF into the DB
./biking_home -extract-osm /path/to/france-latest.osm.pbf

# Backfill OSM coordinates onto passes (needs -extract-osm run first)
./biking_home -enrich

```

## Options / Configuration

| Flag | Description | Default |
| --- | --- | --- |
| `-download` | Download mountain passes into the database | `false` |
| `-resume` | Skip departments already cached on disk | `false` |
| `-import-cached` | Import cached department CSVs into the database | `false` |
| `-extract-osm` | Extract mountain passes from a France OSM PBF | `""` |
| `-enrich` | Backfill OSM coordinates onto passes | `false` |
| `-backfill` | Recompute and persist all ride computed values | `false` |

Configuration is stored in `config.yaml`. Start from `config.example.yaml` and set the Strava credentials before launching the server.

## Strava API

The Strava v3 API (OAuth2) is the platform's public access point; scopes gate each resource (`read`, `activity:read_all`, `activity:write`, `push:subscriptions`). It offers:

- **Activities** — list and fetch ride detail, plus data streams (lat/lng, altitude, time, distance, heart rate, cadence, power) to rebuild GPX or compute metrics
- **Segments** — explore segments, match segment efforts, pull personal records
- **Uploads** — create activities by pushing FIT, GPX or TCX files
- **Webhooks** — push subscriptions notify a callback URL on activity create/delete, enabling real-time sync
- **Extras** — clubs, routes, gear, photos, kudos

Standard apps are rate-limited to 100 calls per 15 minutes and 1,000 per day.

## Architecture

- Go 1.23; SQLite via `mattn/go-sqlite3`
- `ride` — GPX parsing, climb detection, difficulty scores (KOM + Cotacol), similarity index
- `mountain_pass` — centcols.org department CSV download into SQLite, with disk caching and retries
- `osmpass` — OSM PBF extraction (`mountain_pass=yes` nodes) and pass coordinate enrichment
- `strava` — OAuth2 client returning activity metadata and GPX data
- `rides` — SQLite persistence for imported ride metadata and versioned computed values
- `config` — typed YAML configuration and atomic persistence
- `web` — HTTP server, OAuth callback, sync orchestration, and templ pages
- `cli` — legacy mountain-pass and OSM command handlers
- **Notable choices** — the difficulty score follows the Cotacol method: the ride is split into fixed 100 m segments and each scores `distance_km × slope²`, so steep sections weigh exponentially more than long flat ones

```mermaid
flowchart TB
    %% Arrow X --> Y means: X depends on Y

    main["main (bootstrap: main.go)"]

    ride["ride (GPX parsing, climbs, Cotacol)"]
    mpass["mountain_pass (centcols, crossings)"]
    osmpass["osmpass (OSM PBF, enrichment)"]
    strava["strava (OAuth, List, Get)"]
    rides["rides (SQLite persistence)"]
    config["config (YAML)"]
    web["web (HTTP + templ)"]
    cli["cli (legacy commands)"]

    main --> cli
    cli --> ride
    cli --> mpass
    cli --> osmpass
    cli --> web
    mpass --> ride
    web --> strava
    web --> rides
    web --> config
```

## Research

MapLibre can render local vector-tile maps in the browser from OpenStreetMap PBF data after preprocessing. It supports GeoJSON overlays, dynamic styling, interactive points and lines, feature selection, and offline use with locally served tiles, styles, fonts, and sprites. This enables rich map layers without depending on external basemap tile services.

## Development

```bash
mise run check
mise run build
```

## TODO

- Bug: cotacol on global profile differs from cotacol in climbs to match
- Compute estimated power, speed
- Plot speed and slope per segment, colored by heart rate
- Plot speed vs ctc/100km
- Blog with pictures and markdown
- Road quality (Arbois = 2/5, Roquefavour = 4/5)
- OffClimb
    - Keep trace appart from ride to display slope profile, compute cotacol
    - Profile - Adaptative to show ramps
- Planning rides
- Ride details
    - Display mountain pass on map in bounding box
    - pretty climb card, to reuse for official as well as preview and detected
    - Climb on map - display start and end icons

## Resources

- [Climbfinder: difficulty points](https://climbfinder.com/en/difficulty-points) — the Cotacol formula behind the difficulty score
- [centcols.org](https://www.centcols.org) — French mountain pass reference used for the SQLite data
- [Pogacar 21 July 2022](https://www.strava.com/activities/7505784085) — example ride used for climb detection
- [gpxgo](https://github.com/tkrajina/gpxgo) — GPX parsing library
- [geodist](https://github.com/jftuga/geodist) — haversine distance used for climb matching
