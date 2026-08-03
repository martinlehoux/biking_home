package osmpass

import (
	"database/sql"
	"fmt"
	"log/slog"
	"strings"
	"unicode"

	"golang.org/x/text/unicode/norm"
)

const elevationToleranceM = 25

type osmPass struct {
	Name      string
	Elevation *int
	Latitude  float64
	Longitude float64
}

func EnrichMountainPasses(db *sql.DB) (int, error) {
	osmPasses, err := loadOSMPasses(db)
	if err != nil {
		return 0, err
	}
	rows, err := db.Query(`
		SELECT external_id, name, department_code, elevation
		FROM mountain_passes
		WHERE latitude IS NULL
	`)
	if err != nil {
		return 0, err
	}

	unmatched := make([]struct {
		ExternalID     string
		Name           string
		DepartmentCode string
		Elevation      int
	}, 0)
	for rows.Next() {
		var candidate struct {
			ExternalID     string
			Name           string
			DepartmentCode string
			Elevation      int
		}
		if err := rows.Scan(&candidate.ExternalID, &candidate.Name, &candidate.DepartmentCode, &candidate.Elevation); err != nil {
			rows.Close()
			return 0, err
		}
		unmatched = append(unmatched, candidate)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return 0, err
	}
	rows.Close()

	statement, err := db.Prepare(`
		UPDATE mountain_passes SET latitude = ?, longitude = ? WHERE external_id = ?
	`)
	if err != nil {
		return 0, err
	}
	defer statement.Close()

	count := 0
	for _, candidate := range unmatched {
		matched := matchToOSM(candidate.Name, candidate.DepartmentCode, candidate.Elevation, osmPasses)
		if matched == nil {
			continue
		}
		if _, err := statement.Exec(matched.Latitude, matched.Longitude, candidate.ExternalID); err != nil {
			return count, err
		}
		count++
	}
	slog.Info("Enriched mountain passes with coordinates", "count", count)
	return count, nil
}

func loadOSMPasses(db *sql.DB) ([]osmPass, error) {
	rows, err := db.Query(`
		SELECT name, elevation, latitude, longitude
		FROM osm_passes
		WHERE elevation IS NOT NULL
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	osmPasses := make([]osmPass, 0)
	for rows.Next() {
		var mountainPass osmPass
		var name sql.NullString
		var elevation sql.NullInt64
		if err := rows.Scan(&name, &elevation, &mountainPass.Latitude, &mountainPass.Longitude); err != nil {
			return nil, err
		}
		mountainPass.Name = name.String
		if elevation.Valid {
			elevationValue := int(elevation.Int64)
			mountainPass.Elevation = &elevationValue
		}
		osmPasses = append(osmPasses, mountainPass)
	}
	return osmPasses, rows.Err()
}

func matchToOSM(name, departmentCode string, elevation int, osmPasses []osmPass) *osmPass {
	bbox := departmentBBox(departmentCode)
	var best *osmPass
	bestNameMatch := false
	bestElevationDiff := 0.0
	for i := range osmPasses {
		candidate := &osmPasses[i]
		if candidate.Elevation == nil {
			continue
		}
		if !bbox.Contains(candidate.Latitude, candidate.Longitude) {
			continue
		}
		elevationDiff := absFloat64(float64(elevation) - float64(*candidate.Elevation))
		if elevationDiff > elevationToleranceM {
			continue
		}
		nameMatch := nameMatches(name, candidate.Name)
		if best == nil ||
			(nameMatch && !bestNameMatch) ||
			(nameMatch == bestNameMatch && elevationDiff < bestElevationDiff) {
			best = candidate
			bestNameMatch = nameMatch
			bestElevationDiff = elevationDiff
		}
	}
	if best == nil {
		return nil
	}
	if bestNameMatch {
		return best
	}
	for i := range osmPasses {
		candidate := &osmPasses[i]
		if candidate == best || candidate.Elevation == nil {
			continue
		}
		if !bbox.Contains(candidate.Latitude, candidate.Longitude) {
			continue
		}
		if absFloat64(float64(elevation)-float64(*candidate.Elevation)) <= bestElevationDiff+30 {
			return nil
		}
	}
	return best
}

type bbox struct {
	minLat, minLon, maxLat, maxLon float64
}

func (b bbox) Contains(latitude, longitude float64) bool {
	return latitude >= b.minLat && latitude <= b.maxLat &&
		longitude >= b.minLon && longitude <= b.maxLon
}

func departmentBBox(departmentCode string) bbox {
	switch departmentCode {
	case "01":
		return bbox{minLat: 45.5, minLon: 4.7, maxLat: 46.6, maxLon: 6.3}
	case "06":
		return bbox{minLat: 43.5, minLon: 6.4, maxLat: 44.5, maxLon: 7.8}
	case "13":
		return bbox{minLat: 43.1, minLon: 4.4, maxLat: 44.0, maxLon: 6.0}
	default:
		return bbox{minLat: 41.0, minLon: -6.0, maxLat: 52.0, maxLon: 10.0}
	}
}

func nameMatches(centName, osmName string) bool {
	cent := normalizeName(centName)
	osm := normalizeName(osmName)
	if cent == osm {
		return true
	}
	centTokens := tokenize(cent)
	if len(centTokens) < 3 {
		return false
	}
	osmTokens := make(map[string]bool)
	for _, token := range tokenize(osm) {
		osmTokens[token] = true
	}
	for _, token := range centTokens {
		if !osmTokens[token] {
			return false
		}
	}
	return true
}

func tokenize(name string) []string {
	return strings.Fields(name)
}

func normalizeName(name string) string {
	name = strings.ToLower(name)
	name = norm.NFD.String(name)
	normalized := strings.Builder{}
	previousSpace := false
	for _, r := range name {
		if unicode.Is(unicode.Mn, r) {
			continue
		}
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			normalized.WriteRune(r)
			previousSpace = false
		} else if !previousSpace {
			normalized.WriteRune(' ')
			previousSpace = true
		}
	}
	return strings.TrimSpace(normalized.String())
}

func absFloat64(value float64) float64 {
	if value < 0 {
		return -value
	}
	return value
}

func (p osmPass) String() string {
	if p.Elevation == nil {
		return fmt.Sprintf("%s @ unknown elevation", p.Name)
	}
	return fmt.Sprintf("%s @ %dm", p.Name, *p.Elevation)
}
