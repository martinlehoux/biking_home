package osmpass

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strconv"
	"strings"

	"github.com/paulmach/osm"
	"github.com/paulmach/osm/osmpbf"
)

const insertStatement = `
	INSERT INTO osm_passes (osm_id, name, elevation, latitude, longitude)
	VALUES (?, ?, ?, ?, ?)
	ON CONFLICT(osm_id) DO UPDATE SET
		name = excluded.name,
		elevation = excluded.elevation,
		latitude = excluded.latitude,
		longitude = excluded.longitude
`

func ExtractMountainPasses(ctx context.Context, pbfPath string, db *sql.DB) (int, error) {
	file, err := os.Open(pbfPath)
	if err != nil {
		return 0, fmt.Errorf("failed to open pbf file: %w", err)
	}
	defer file.Close()

	scanner := osmpbf.New(ctx, file, 4)
	scanner.SkipWays = true
	scanner.SkipRelations = true
	scanner.FilterNode = isMountainPass

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	stmt, err := tx.PrepareContext(ctx, insertStatement)
	if err != nil {
		return 0, fmt.Errorf("failed to prepare insert: %w", err)
	}
	defer stmt.Close()

	count := 0
	for scanner.Scan() {
		node, ok := scanner.Object().(*osm.Node)
		if !ok {
			continue
		}
		elevation, hasElevation := parseElevation(node.Tags.Find("ele"))
		var elevationValue any
		if hasElevation {
			elevationValue = elevation
		}
		if _, err := stmt.ExecContext(ctx, node.ID, node.Tags.Find("name"), elevationValue, node.Lat, node.Lon); err != nil {
			return count, fmt.Errorf("failed to insert osm pass %d: %w", node.ID, err)
		}
		count++
	}
	if err := scanner.Err(); err != nil && err != io.EOF {
		return count, fmt.Errorf("failed to scan pbf: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return count, fmt.Errorf("failed to commit transaction: %w", err)
	}
	slog.Info("Extracted mountain passes", "count", count, "source", pbfPath)
	return count, nil
}

func isMountainPass(node *osm.Node) bool {
	if node.Tags.Find("mountain_pass") == "yes" {
		return true
	}
	return node.Tags.Find("natural") == "mountain_pass"
}

func parseElevation(ele string) (int, bool) {
	ele = strings.TrimSpace(ele)
	if ele == "" {
		return 0, false
	}
	for _, candidate := range strings.Fields(ele) {
		if value, err := strconv.Atoi(candidate); err == nil {
			return value, true
		}
		if value, err := strconv.ParseFloat(candidate, 64); err == nil {
			return int(value), true
		}
	}
	return 0, false
}
