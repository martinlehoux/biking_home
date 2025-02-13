# Bike software

## Features

- Handling my historical data
- Blog with pictures and markdown
- Storing GPX
- Export mountain pass from OSM and auto detect when pass is crossed
- Auto detect climbs from GPX
- Compute estimated power
- Export data from Strava, Garmin
  - Garmin GPX exports has elevation, time, temp, heart rate, cadence
  - Garmin TCX export has stats (cal, heart rate, time, cadence), time, pos, alt, distance, hr, cadence, speed
  - Garmin Fit export has (time, sport, lap/split, gps, sensor, events)?
  - KML ?

## Implementation

- Golang for showcase
- Keep improving personal lib (+ documentation)
- Get back what is interesting in previous projects
  - Go bike:
    - Only tooling for golang, project is different
  - Django bike
- https://github.com/paulmach/osm
- https://github.com/tkrajina/gpxgo
- https://github.com/tormoder/fit
- https://github.com/brendangregg/FlameGraph
- `go tool pprof -http=":8081"`

## TODO

- CI / Tooling
- Tests
- linter for calls to `make([]..., n)` that then use append

## Examples

**[Pogacar 21 july 2022](https://www.strava.com/activities/7505784085)**
