package official_climb

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/jftuga/geodist"
)

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
