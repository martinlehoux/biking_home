package osmpass

import (
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"

	"github.com/schollz/progressbar/v3"
)

const francePBFRUL = "https://download.geofabrik.de/europe/france-latest.osm.pbf"

// FetchFrancePBF downloads the full France OSM PBF to destPath, resuming a
// partial download when the file already exists and skipping the download
// entirely when it is already complete.
func FetchFrancePBF(destPath string) error {
	client := &http.Client{}

	head, err := client.Head(francePBFRUL)
	if err != nil {
		return fmt.Errorf("failed to check remote size: %w", err)
	}
	head.Body.Close()
	total := head.ContentLength
	if total <= 0 {
		return fmt.Errorf("unexpected remote content length: %d", total)
	}

	info, statErr := os.Stat(destPath)
	var offset int64
	if statErr == nil {
		offset = info.Size()
		if offset >= total {
			slog.Info("Already downloaded", "file", destPath, "bytes", offset)
			return nil
		}
		if offset > 0 {
			slog.Info("Resuming download", "file", destPath, "bytes", offset, "remaining", total-offset)
		}
	} else if !errors.Is(statErr, os.ErrNotExist) {
		return fmt.Errorf("failed to stat %s: %w", destPath, statErr)
	}

	req, err := http.NewRequest(http.MethodGet, francePBFRUL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "biking_home")
	if offset > 0 {
		req.Header.Set("Range", fmt.Sprintf("bytes=%d-", offset))
	}

	res, err := client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	resuming := offset > 0 && res.StatusCode == http.StatusPartialContent
	if !resuming {
		if res.StatusCode != http.StatusOK {
			return fmt.Errorf("unexpected status code: %d", res.StatusCode)
		}
		if offset > 0 {
			slog.Warn("Server ignored Range header, restarting download", "file", destPath)
		}
		offset = 0
	}

	mode := os.O_CREATE | os.O_WRONLY | os.O_TRUNC
	if resuming {
		mode = os.O_CREATE | os.O_WRONLY
	}
	file, err := os.OpenFile(destPath, mode, 0o644)
	if err != nil {
		return err
	}
	defer file.Close()

	bar := progressbar.DefaultBytes(res.ContentLength, "Downloading france-latest.osm.pbf")
	if resuming {
		// Re-append to the partial file when the server honored the range request.
		if _, err := file.Seek(offset, io.SeekStart); err != nil {
			return err
		}
	}
	_, err = io.Copy(io.MultiWriter(file, bar), res.Body)
	if err != nil {
		return fmt.Errorf("failed to download %s: %w", francePBFRUL, err)
	}
	if size, err := file.Stat(); err == nil && size.Size() != total {
		return fmt.Errorf("download incomplete: got %d bytes, want %d", size.Size(), total)
	}
	return nil
}
