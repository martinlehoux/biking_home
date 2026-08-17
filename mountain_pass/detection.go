package mountain_pass

import (
	"database/sql"
	"fmt"
	"math"

	"github.com/jftuga/geodist"
	"github.com/martinlehoux/biking_home/ride"
)

type Crossing struct {
	Pass          MountainPass
	DistanceToM   float64
	RideDistanceM float64
	RideElevation float64
	ElevationDiff float64
}

func LoadMountainPasses(db *sql.DB) ([]MountainPass, error) {
	return loadMountainPasses(db, nil)
}

func LoadMountainPassesAroundRide(db *sql.DB, route ride.Ride, marginM float64) ([]MountainPass, error) {
	return loadMountainPasses(db, rideBounds(route, marginM))
}

type coordinateBounds struct {
	minLatitude  float64
	minLongitude float64
	maxLatitude  float64
	maxLongitude float64
}

func rideBounds(route ride.Ride, marginM float64) *coordinateBounds {
	minLatitude, maxLatitude := route.Coord(0).Lat, route.Coord(0).Lat
	minLongitude, maxLongitude := route.Coord(0).Lon, route.Coord(0).Lon
	for index := 1; index < route.Len(); index++ {
		coordinate := route.Coord(index)
		minLatitude = math.Min(minLatitude, coordinate.Lat)
		maxLatitude = math.Max(maxLatitude, coordinate.Lat)
		minLongitude = math.Min(minLongitude, coordinate.Lon)
		maxLongitude = math.Max(maxLongitude, coordinate.Lon)
	}
	latitudeMargin := marginM / 111_320
	longitudeScale := math.Cos((minLatitude + maxLatitude) / 2 * math.Pi / 180)
	longitudeMargin := marginM / (111_320 * math.Max(longitudeScale, 0.01))
	return &coordinateBounds{
		minLatitude:  minLatitude - latitudeMargin,
		minLongitude: minLongitude - longitudeMargin,
		maxLatitude:  maxLatitude + latitudeMargin,
		maxLongitude: maxLongitude + longitudeMargin,
	}
}

func loadMountainPasses(db *sql.DB, bounds *coordinateBounds) ([]MountainPass, error) {
	query := `
		SELECT external_id, name, country_code, department_code, elevation, latitude, longitude
		FROM mountain_passes`
	args := make([]any, 0, 4)
	if bounds != nil {
		query += `
		WHERE latitude IS NOT NULL AND longitude IS NOT NULL
		  AND latitude BETWEEN ? AND ?
		  AND longitude BETWEEN ? AND ?`
		args = append(args, bounds.minLatitude, bounds.maxLatitude, bounds.minLongitude, bounds.maxLongitude)
	}
	query += `
		ORDER BY elevation`
	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	mountainPasses := make([]MountainPass, 0)
	for rows.Next() {
		var mountainPass MountainPass
		var latitude, longitude sql.NullFloat64
		if err := rows.Scan(&mountainPass.ExternalID, &mountainPass.Name, &mountainPass.CountryCode, &mountainPass.DepartmentCode, &mountainPass.Elevation, &latitude, &longitude); err != nil {
			return nil, err
		}
		if latitude.Valid && longitude.Valid {
			mountainPass.Coord = &geodist.Coord{Lat: latitude.Float64, Lon: longitude.Float64}
		}
		mountainPasses = append(mountainPasses, mountainPass)
	}
	return mountainPasses, rows.Err()
}

func DetectCrossings(ride ride.Ride, passes []MountainPass, radiusM, elevationToleranceM float64) []Crossing {
	crossings := make([]Crossing, 0)
	for _, mountainPass := range passes {
		if mountainPass.Coord == nil {
			continue
		}
		crossing, found := nearestCrossing(ride, mountainPass)
		if found && crossing.DistanceToM <= radiusM && crossing.ElevationDiff <= elevationToleranceM {
			crossings = append(crossings, crossing)
		}
	}
	return crossings
}

// MatchClimb returns the pass whose coordinates lie within radiusM of the
// climb's highest point and whose elevation is within elevationToleranceM of
// that point's elevation, nearest first. Found is false when no pass matches.
func MatchClimb(climb ride.Climb, passes []MountainPass, radiusM, elevationToleranceM float64) (MountainPass, bool) {
	topCoord := climb.TopCoord()
	topElevation := climb.TopElevationM()
	var best MountainPass
	bestDistanceM := radiusM
	found := false
	for _, mountainPass := range passes {
		if mountainPass.Coord == nil {
			continue
		}
		distanceKm, _ := geodist.HaversineDistance(topCoord, *mountainPass.Coord)
		distanceM := distanceKm * 1000
		if distanceM > bestDistanceM {
			continue
		}
		elevationDiff := absFloat64(topElevation - float64(mountainPass.Elevation))
		if elevationDiff > elevationToleranceM {
			continue
		}
		best = mountainPass
		bestDistanceM = distanceM
		found = true
	}
	return best, found
}

func nearestCrossing(ride ride.Ride, mountainPass MountainPass) (Crossing, bool) {
	best := Crossing{Pass: mountainPass, DistanceToM: 1e18}
	for i := 0; i < ride.Len(); i++ {
		distanceKm, _ := geodist.HaversineDistance(ride.Coord(i), *mountainPass.Coord)
		distanceM := distanceKm * 1000
		if distanceM < best.DistanceToM {
			best.DistanceToM = distanceM
			best.RideDistanceM = ride.DistanceM(i)
			best.RideElevation = ride.ElevationM(i)
			best.ElevationDiff = absFloat64(ride.ElevationM(i) - float64(mountainPass.Elevation))
		}
	}
	if best.DistanceToM > 1e17 {
		return best, false
	}
	return best, true
}

func absFloat64(value float64) float64 {
	if value < 0 {
		return -value
	}
	return value
}

func (crossing Crossing) String() string {
	return fmt.Sprintf("%s (%dm) at %.1fkm, %.0fm away, Δelev %.0fm",
		crossing.Pass.Name, crossing.Pass.Elevation, crossing.RideDistanceM/1000, crossing.DistanceToM, crossing.ElevationDiff)
}
