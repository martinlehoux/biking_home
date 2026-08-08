package rides

import (
	"database/sql"
	"fmt"
	"log/slog"
	"time"

	"github.com/martinlehoux/biking_home/ride"
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
	cotacolScore        *float64
	cotacolAlgoVersion  string
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
	SortCotacol    SortColumn = "cotacol"
	SortCotacolKm  SortColumn = "cotacol_100km"
)

const columns = "id, external_id, gpx_path, name, type, start_date, distance_m, moving_time_s, elapsed_time_s, total_elevation_gain_m, average_speed_mps, cotacol_score, cotacol_algo_version, created_at, updated_at"

func Save(db *sql.DB, r Ride) error {
	parsed, err := ride.ParseFile(ride.GPXRideParser{}, r.GPXPath)
	if err != nil {
		return fmt.Errorf("compute Cotacol for ride %q: %w", r.ExternalID, err)
	}
	cotacolScore := ride.Cotacol(parsed)
	_, err = db.Exec(`
		INSERT INTO rides (external_id, gpx_path, name, type, start_date, distance_m, moving_time_s, elapsed_time_s, total_elevation_gain_m, average_speed_mps, cotacol_score, cotacol_algo_version)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
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
			cotacol_score = excluded.cotacol_score,
			cotacol_algo_version = excluded.cotacol_algo_version,
			updated_at = strftime('%Y-%m-%dT%H:%M:%SZ', 'now')
	`, r.ExternalID, r.GPXPath, r.Name, r.Type, r.StartDate.UTC().Format(time.RFC3339), r.DistanceM, r.MovingTimeS, r.ElapsedTimeS, r.TotalElevationGainM, r.AverageSpeedMps, cotacolScore, ride.CotacolAlgorithmVersion)
	return err
}

func Backfill(db *sql.DB) (int, error) {
	rows, err := db.Query("SELECT " + columns + " FROM rides ORDER BY id")
	if err != nil {
		return 0, err
	}
	var pending []Ride
	for rows.Next() {
		item, err := scanRide(rows)
		if err != nil {
			rows.Close()
			return 0, err
		}
		pending = append(pending, item)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return 0, err
	}
	if err := rows.Close(); err != nil {
		return 0, err
	}
	backfilled := 0
	for _, item := range pending {
		if err := Save(db, item); err != nil {
			slog.Warn("Failed to backfill ride values", "ride", item.ExternalID, "file", item.GPXPath, "error", err)
			continue
		}
		backfilled++
	}
	return backfilled, nil
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
	query := "SELECT " + columns + " FROM rides ORDER BY "
	args := []any{}
	if isComputedSort(column) {
		query += "(cotacol_score IS NULL OR cotacol_algo_version IS NULL OR cotacol_algo_version <> ? OR distance_m <= 0) ASC, "
		args = append(args, ride.CotacolAlgorithmVersion)
	}
	query += expression + " " + direction + ", id DESC"
	rows, err := db.Query(query, args...)
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
	SortCotacol:    "cotacol_score",
	SortCotacolKm:  "cotacol_score * 100000.0 / NULLIF(distance_m, 0)",
}

func isComputedSort(column SortColumn) bool {
	return column == SortCotacol || column == SortCotacolKm
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
		ride               Ride
		startDate          string
		cotacolScore       sql.NullFloat64
		cotacolAlgoVersion sql.NullString
		createdAt          string
		updatedAt          string
	)
	err := s.Scan(&ride.ID, &ride.ExternalID, &ride.GPXPath, &ride.Name, &ride.Type, &startDate, &ride.DistanceM, &ride.MovingTimeS, &ride.ElapsedTimeS, &ride.TotalElevationGainM, &ride.AverageSpeedMps, &cotacolScore, &cotacolAlgoVersion, &createdAt, &updatedAt)
	if err != nil {
		return Ride{}, err
	}
	if cotacolScore.Valid {
		ride.cotacolScore = &cotacolScore.Float64
	}
	if cotacolAlgoVersion.Valid {
		ride.cotacolAlgoVersion = cotacolAlgoVersion.String
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

func (r Ride) CotacolAlgorithmVersion() string {
	return r.cotacolAlgoVersion
}

func (r Ride) CotacolScore() (float64, bool) {
	if r.cotacolScore == nil {
		return 0, false
	}
	return *r.cotacolScore, true
}
