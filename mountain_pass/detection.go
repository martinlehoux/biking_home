package mountain_pass

import (
	"database/sql"
	"fmt"

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
	rows, err := db.Query(`
		SELECT external_id, name, country_code, department_code, elevation, latitude, longitude
		FROM mountain_passes
		ORDER BY elevation
	`)
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
	top := climb.Top()
	var best MountainPass
	bestDistanceM := radiusM
	found := false
	for _, mountainPass := range passes {
		if mountainPass.Coord == nil {
			continue
		}
		distanceKm, _ := geodist.HaversineDistance(top.Coord, *mountainPass.Coord)
		distanceM := distanceKm * 1000
		if distanceM > bestDistanceM {
			continue
		}
		elevationDiff := absFloat64(top.ElevationM - float64(mountainPass.Elevation))
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
	for _, point := range ride.Points() {
		distanceKm, _ := geodist.HaversineDistance(point.Coord, *mountainPass.Coord)
		distanceM := distanceKm * 1000
		if distanceM < best.DistanceToM {
			best.DistanceToM = distanceM
			best.RideDistanceM = point.DistanceM
			best.RideElevation = point.ElevationM
			best.ElevationDiff = absFloat64(point.ElevationM - float64(mountainPass.Elevation))
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
