# Bike software

## Features

- [ ] Detect similar climbs + PRs
- On this climb (examples/activity_18679866717.gpx), the climb algo should detect several possible climbs and let you choose
- Once you have chosen, new activities should have remembered the choice (looking into the index should be enough)
- Handling my historical data
- Blog with pictures and markdown
- Storing GPX
- Export mountain pass from OSM and auto detect when pass is crossed
  - https://www.jmspae.se/write-ups/kebabs-train-stations/
- Auto detect climbs from GPX
- Compute estimated power
- Export data from Strava, Garmin
  - Garmin GPX exports has elevation, time, temp, heart rate, cadence
  - Garmin TCX export has stats (cal, heart rate, time, cadence), time, pos, alt, distance, hr, cadence, speed
  - Garmin Fit export has (time, sport, lap/split, gps, sensor, events)?
  - KML ?
- Strava seems the simpler to use

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
- https://developer.garmin.com/gc-developer-program/activity-api/
- https://github.com/matin/garth
- https://developers.strava.com/docs/webhooks/
- https://developers.strava.com/docs/getting-started/
- https://connect.garmin.com/modern/activities to CSV
- https://github.com/SamR1/FitTrackee
- https://github.com/jovandeginste/workout-tracker

## TODO

- CI / Tooling
- Tests
- linter for calls to `make([]..., n)` that then use append

## Examples

**[Pogacar 21 july 2022](https://www.strava.com/activities/7505784085)**
