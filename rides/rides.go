package rides

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/martinlehoux/kagamigo/kcore"
)

type Ride struct {
	ID                  int64
	ExternalID          string
	GPXPath             string
	Name                string
	Type                string
	StartDate           time.Time
	DistanceM           float64
	MovingTimeS         int64
	ElapsedTimeS        int64
	TotalElevationGainM float64
	AverageSpeedMps     float64
	CreatedAt           time.Time
	UpdatedAt           time.Time
}

type SortColumn string

const (
	SortName       SortColumn = "name"
	SortStartDate  SortColumn = "started"
	SortDistance   SortColumn = "distance"
	SortMovingTime SortColumn = "moving_time"
	SortElevation  SortColumn = "elevation"
)

const columns = "id, external_id, gpx_path, name, type, start_date, distance_m, moving_time_s, elapsed_time_s, total_elevation_gain_m, average_speed_mps, created_at, updated_at"

func Save(db *sql.DB, r Ride) error {
	_, err := db.Exec(`
		INSERT INTO rides (external_id, gpx_path, name, type, start_date, distance_m, moving_time_s, elapsed_time_s, total_elevation_gain_m, average_speed_mps)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(external_id) DO UPDATE SET
			gpx_path = excluded.gpx_path,
			name = excluded.name,
			type = excluded.type,
			start_date = excluded.start_date,
			distance_m = excluded.distance_m,
			moving_time_s = excluded.moving_time_s,
			elapsed_time_s = excluded.elapsed_time_s,
			total_elevation_gain_m = excluded.total_elevation_gain_m,
			average_speed_mps = excluded.average_speed_mps,
			updated_at = strftime('%Y-%m-%dT%H:%M:%SZ', 'now')
	`, r.ExternalID, r.GPXPath, r.Name, r.Type, r.StartDate.UTC().Format(time.RFC3339), r.DistanceM, r.MovingTimeS, r.ElapsedTimeS, r.TotalElevationGainM, r.AverageSpeedMps)
	return err
}

func List(db *sql.DB) ([]Ride, error) {
	return ListSorted(db, SortStartDate, true)
}

func ListSorted(db *sql.DB, column SortColumn, descending bool) ([]Ride, error) {
	expression, ok := sortExpressions[column]
	if !ok {
		return nil, fmt.Errorf("invalid ride sort column %q", column)
	}
	direction := "ASC"
	if descending {
		direction = "DESC"
	}
	rows, err := db.Query("SELECT " + columns + " FROM rides ORDER BY " + expression + " " + direction + ", id DESC")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var rides []Ride
	for rows.Next() {
		ride, err := scanRide(rows)
		if err != nil {
			return nil, err
		}
		rides = append(rides, ride)
	}
	return rides, rows.Err()
}

var sortExpressions = map[SortColumn]string{
	SortName:       "name COLLATE NOCASE",
	SortStartDate:  "start_date",
	SortDistance:   "distance_m",
	SortMovingTime: "moving_time_s",
	SortElevation:  "total_elevation_gain_m",
}

func GetByExternalID(db *sql.DB, externalID string) (Ride, bool, error) {
	row := db.QueryRow("SELECT "+columns+" FROM rides WHERE external_id = ?", externalID)
	ride, err := scanRide(row)
	if err == sql.ErrNoRows {
		return Ride{}, false, nil
	}
	if err != nil {
		return Ride{}, false, err
	}
	return ride, true, nil
}

type scanner interface {
	Scan(dest ...any) error
}

func scanRide(s scanner) (Ride, error) {
	var (
		ride      Ride
		startDate string
		createdAt string
		updatedAt string
	)
	err := s.Scan(&ride.ID, &ride.ExternalID, &ride.GPXPath, &ride.Name, &ride.Type, &startDate, &ride.DistanceM, &ride.MovingTimeS, &ride.ElapsedTimeS, &ride.TotalElevationGainM, &ride.AverageSpeedMps, &createdAt, &updatedAt)
	if err != nil {
		return Ride{}, err
	}
	ride.StartDate, err = time.Parse(time.RFC3339, startDate)
	if err != nil {
		return Ride{}, kcore.Wrap(err, "invalid start_date in rides row")
	}
	ride.CreatedAt, err = time.Parse(time.RFC3339, createdAt)
	if err != nil {
		return Ride{}, kcore.Wrap(err, "invalid created_at in rides row")
	}
	ride.UpdatedAt, err = time.Parse(time.RFC3339, updatedAt)
	if err != nil {
		return Ride{}, kcore.Wrap(err, "invalid updated_at in rides row")
	}
	return ride, nil
}
