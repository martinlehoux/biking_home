package mountainpass

import (
	"bytes"
	"database/sql"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/martinlehoux/kagamigo/kcore"
	"github.com/schollz/progressbar/v3"
)

type MountainPass struct {
	Name           string
	ExternalID     string
	CountryCode    string
	DepartmentCode string
	Elevation      int
}

func parseMountainPasses(reader io.Reader) ([]MountainPass, error) {
	mountainPasses := make([]MountainPass, 0)
	csvReader := csv.NewReader(reader)
	csvReader.Comma = '\t'
	csvReader.LazyQuotes = true
	rows, err := csvReader.ReadAll()
	if err != nil {
		return nil, kcore.Wrap(err, "Failed to read mountain pass CSV data")
	}
	rows = rows[1:]
	for _, row := range rows {
		if len(row) < 4 {
			return nil, fmt.Errorf("unexpected row with %d fields: %v", len(row), row)
		}
		countryCode, departmentCode, err := splitCountryDepartment(row[0])
		if err != nil {
			return nil, err
		}
		codes := strings.Split(row[1], " | ")
		names := strings.Split(row[2], " | ")
		altitudes := strings.Split(row[3], " | ")
		if len(codes) != len(names) || len(codes) != len(altitudes) {
			return nil, fmt.Errorf("inconsistent multi-pass row: %v", row)
		}
		for i := range codes {
			elevation, err := strconv.Atoi(altitudes[i])
			if err != nil {
				return nil, fmt.Errorf("failed to parse elevation %q: %w", altitudes[i], err)
			}
			mountainPasses = append(mountainPasses, MountainPass{
				ExternalID:     "centcols/" + codes[i],
				CountryCode:    countryCode,
				Name:           names[i],
				DepartmentCode: departmentCode,
				Elevation:      elevation,
			})
		}
	}

	return mountainPasses, nil
}

func splitCountryDepartment(brevet string) (string, string, error) {
	parts := strings.Split(brevet, "-")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", fmt.Errorf("unexpected brevet code: %q", brevet)
	}
	return parts[0], parts[1], nil
}

func getCSVData(departmentCode string) ([]byte, error) {
	url := "https://www.centcols.org/membres/cols/csvbrevet.php?id=FR" + departmentCode + "&enc=UTF-8"
	res, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return nil, statusError{statusCode: res.StatusCode}
	}
	return io.ReadAll(res.Body)
}

type statusError struct {
	statusCode int
}

func (e statusError) Error() string {
	return fmt.Sprintf("unexpected status code: %d", e.statusCode)
}

const downloadRetries = 3

func departmentCacheFile(departmentCode string) string {
	return fmt.Sprintf("debug_department_%s.csv", departmentCode)
}

func loadCachedDepartment(filename string) ([]MountainPass, bool, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, false, nil
		}
		return nil, false, err
	}
	mountainPasses, err := parseMountainPasses(bytes.NewReader(data))
	if err != nil {
		return nil, false, nil
	}
	return mountainPasses, true, nil
}

func downloadDepartment(departmentCode string, resume bool) ([]MountainPass, error) {
	filename := departmentCacheFile(departmentCode)
	if resume {
		mountainPasses, found, err := loadCachedDepartment(filename)
		if err != nil {
			return nil, err
		}
		if found {
			return mountainPasses, nil
		}
	}
	var lastErr error
	backoff := time.Second
	for attempt := 1; attempt <= downloadRetries; attempt++ {
		data, err := getCSVData(departmentCode)
		if err != nil {
			lastErr = err
			var statusErr statusError
			if errors.As(err, &statusErr) && statusErr.statusCode == http.StatusTooManyRequests {
				backoff = time.Minute
			}
			slog.Warn("Retrying department download", "department", departmentCode, "attempt", attempt, "error", err)
			time.Sleep(backoff)
			backoff *= 2
			continue
		}
		mountainPasses, err := parseMountainPasses(bytes.NewReader(data))
		if err != nil {
			if writeErr := os.WriteFile(filename, data, 0644); writeErr != nil {
				return nil, kcore.Wrap(writeErr, "Failed to write dump file")
			}
			return nil, err
		}
		if writeErr := os.WriteFile(filename, data, 0644); writeErr != nil {
			return nil, kcore.Wrap(writeErr, "Failed to write dump file")
		}
		return mountainPasses, nil
	}
	return nil, lastErr
}

func DownloadMountainPasses(db *sql.DB, delay time.Duration, resume bool) error {
	mountainPasses := make([]MountainPass, 0)
	bar := progressbar.Default(90)
	ticker := time.NewTicker(delay)
	for i := 1; i <= 90; i++ {
		<-ticker.C
		bar.Add(1)
		departmentCode := fmt.Sprintf("%02d", i)
		bar.Describe(fmt.Sprintf("Downloading mountain passes for department_code=%s", departmentCode))
		departmentMountainPasses, err := downloadDepartment(departmentCode, resume)
		if err != nil {
			return kcore.Wrap(err, "Failed to download mountain passes for department "+departmentCode)
		}
		mountainPasses = append(mountainPasses, departmentMountainPasses...)
	}

	tx, err := db.Begin()
	kcore.Expect(err, "Failed to begin transaction")
	defer tx.Rollback()
	stmt, err := tx.Prepare(`
        INSERT INTO mountain_passes (external_id, name, country_code, department_code, elevation)
        VALUES (?, ?, ?, ?, ?)
        ON CONFLICT(external_id) DO UPDATE SET
            name = excluded.name,
            country_code = excluded.country_code,
            department_code = excluded.department_code,
            elevation = excluded.elevation
    `)
	kcore.Expect(err, "Failed to prepare statement")
	defer stmt.Close()

	for _, mountainPass := range mountainPasses {
		_, err := stmt.Exec(mountainPass.ExternalID, mountainPass.Name, mountainPass.CountryCode, mountainPass.DepartmentCode, mountainPass.Elevation)
		kcore.Expect(err, "Failed to execute statement")
	}

	return tx.Commit()
}
