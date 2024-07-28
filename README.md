# Bike software

## Features

- Handling my historical data
- Blog with pictures and markdown
- Storing GPX
- Export mountain pass from OSM and auto detect when pass is crossed
- Auto detect climbs from GPX
- Compute estimated power
- Export data from Strava, Garmin

## Implementation

- Golang for showcase
- Keep improving personal lib (+ documentation)
- Get back what is interesting in previous projects
  - Go bike:
    - Only tooling for golang, project is different
  - Rust bike
    - Algorithm + example
  - Django bike
- https://github.com/paulmach/osm
- https://github.com/tkrajina/gpxgo
- https://github.com/brendangregg/FlameGraph
- `go tool pprof -http=":8081"`

## TODO

- CI / Tooling
- Tests
- linter for calls to `make([]..., n)` that then use append

## Examples

**[Pogacar 21 july 2022](https://www.strava.com/activities/7505784085)**

**Expected**

| From    | To      |
| ------- | ------- |
| 1.0km   | 5.7km   |
| 44.0km  | 45.2km  |
| 60.1km  | 76.3km  |
| 83.8km  | 86.3km  |
| 99.0km  | 109.5km |
| 128.2km | 141.8km |

- add min slope to prevent long false up ?
- check score of 60km-76km vs 32km-76km
  - as expected, the score is better but not found
