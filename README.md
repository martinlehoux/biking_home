# Bike software

## Features

- Handling my historical data
- Blog with pictures and markdown
- Storing GPX
- Export mountain pass from OSM and auto detect when pass is crossed
- Auto detect climbs from GPX

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

## TODO

- CI / Tooling
- Tests

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

- add min slope to prevent long false up
