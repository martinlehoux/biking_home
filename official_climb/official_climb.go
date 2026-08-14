package official_climb

import (
	"database/sql"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/jftuga/geodist"
	"github.com/martinlehoux/biking_home/ride"
)

const DefaultMatchRadiusM = 100.0

type OfficialClimb struct {
	ID         int64
	Name       string
	StartCoord geodist.Coord
	EndCoord   geodist.Coord
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

type MatchPolicy struct {
	EndpointRadiusM float64
}

func DefaultMatchPolicy() MatchPolicy {
	return MatchPolicy{EndpointRadiusM: DefaultMatchRadiusM}
}

func (policy MatchPolicy) Validate() error {
	if math.IsNaN(policy.EndpointRadiusM) || math.IsInf(policy.EndpointRadiusM, 0) || policy.EndpointRadiusM <= 0 {
		return fmt.Errorf("official climb endpoint radius must be greater than zero")
	}
	return nil
}

func (climb OfficialClimb) validate() error {
	if strings.TrimSpace(climb.Name) == "" {
		return fmt.Errorf("official climb name is required")
	}
	if !validCoord(climb.StartCoord) || !validCoord(climb.EndCoord) {
		return fmt.Errorf("official climb coordinates are invalid")
	}
	return nil
}

func validCoord(coord geodist.Coord) bool {
	return coord.Lat >= -90 && coord.Lat <= 90 && coord.Lon >= -180 && coord.Lon <= 180
}

const columns = "id, name, start_latitude, start_longitude, end_latitude, end_longitude, created_at, updated_at"

func Create(db *sql.DB, climb OfficialClimb) (OfficialClimb, error) {
	if err := climb.validate(); err != nil {
		return OfficialClimb{}, err
	}
	result, err := db.Exec(`
		INSERT INTO official_climbs (name, start_latitude, start_longitude, end_latitude, end_longitude)
		VALUES (?, ?, ?, ?, ?)
	`, climb.Name, climb.StartCoord.Lat, climb.StartCoord.Lon, climb.EndCoord.Lat, climb.EndCoord.Lon)
	if err != nil {
		return OfficialClimb{}, fmt.Errorf("create official climb: %w", err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		return OfficialClimb{}, fmt.Errorf("read official climb id: %w", err)
	}
	created, found, err := GetByID(db, id)
	if err != nil {
		return OfficialClimb{}, err
	}
	if !found {
		return OfficialClimb{}, fmt.Errorf("official climb %d was not found after creation", id)
	}
	return created, nil
}

func List(db *sql.DB) ([]OfficialClimb, error) {
	rows, err := db.Query("SELECT " + columns + " FROM official_climbs ORDER BY id")
	if err != nil {
		return nil, fmt.Errorf("list official climbs: %w", err)
	}
	defer rows.Close()

	climbs := make([]OfficialClimb, 0)
	for rows.Next() {
		climb, err := scan(rows)
		if err != nil {
			return nil, err
		}
		climbs = append(climbs, climb)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate official climbs: %w", err)
	}
	return climbs, nil
}

func GetByID(db *sql.DB, id int64) (OfficialClimb, bool, error) {
	row := db.QueryRow("SELECT "+columns+" FROM official_climbs WHERE id = ?", id)
	climb, err := scan(row)
	if err == sql.ErrNoRows {
		return OfficialClimb{}, false, nil
	}
	if err != nil {
		return OfficialClimb{}, false, fmt.Errorf("get official climb %d: %w", id, err)
	}
	return climb, true, nil
}

type scanner interface {
	Scan(dest ...any) error
}

func scan(s scanner) (OfficialClimb, error) {
	var (
		climb                         OfficialClimb
		startLatitude, startLongitude float64
		endLatitude, endLongitude     float64
		createdAt, updatedAt          string
	)
	if err := s.Scan(&climb.ID, &climb.Name, &startLatitude, &startLongitude, &endLatitude, &endLongitude, &createdAt, &updatedAt); err != nil {
		return OfficialClimb{}, err
	}
	climb.StartCoord = geodist.Coord{Lat: startLatitude, Lon: startLongitude}
	climb.EndCoord = geodist.Coord{Lat: endLatitude, Lon: endLongitude}
	var err error
	climb.CreatedAt, err = time.Parse(time.RFC3339, createdAt)
	if err != nil {
		return OfficialClimb{}, fmt.Errorf("invalid official_climbs.created_at: %w", err)
	}
	climb.UpdatedAt, err = time.Parse(time.RFC3339, updatedAt)
	if err != nil {
		return OfficialClimb{}, fmt.Errorf("invalid official_climbs.updated_at: %w", err)
	}
	return climb, nil
}

func MatchClimb(climb ride.Climb, officialClimbs []OfficialClimb, policy MatchPolicy) (OfficialClimb, bool) {
	if policy.Validate() != nil {
		return OfficialClimb{}, false
	}
	bestDistance := math.Inf(1)
	var best OfficialClimb
	found := false
	for _, official := range officialClimbs {
		startIndex, startDistance := nearestClimbPoint(climb, official.StartCoord)
		endIndex, endDistance := nearestClimbPoint(climb, official.EndCoord)
		if startDistance > policy.EndpointRadiusM || endDistance > policy.EndpointRadiusM {
			continue
		}
		if startIndex >= endIndex {
			continue
		}
		totalDistance := startDistance + endDistance
		if totalDistance < bestDistance || (totalDistance == bestDistance && (!found || official.ID < best.ID)) {
			best = official
			bestDistance = totalDistance
			found = true
		}
	}
	return best, found
}

func nearestClimbPoint(climb ride.Climb, target geodist.Coord) (int, float64) {
	bestIndex := climb.StartIndex()
	bestDistance := math.Inf(1)
	for index := climb.StartIndex(); index <= climb.EndIndex(); index++ {
		distance := distanceM(climb.PointCoord(index), target)
		if distance < bestDistance {
			bestIndex = index
			bestDistance = distance
		}
	}
	return bestIndex, bestDistance
}

func distanceM(a, b geodist.Coord) float64 {
	_, distanceKm := geodist.HaversineDistance(a, b)
	return distanceKm * 1000
}
