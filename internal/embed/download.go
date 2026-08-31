package embed

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
)

// Downloader is the injectable download function used by the CLI. Tests may
// override it to avoid network access. Defaults to Download.
var Downloader func(name, modelPath string) (string, error) = Download

// Download downloads the model file for name from ModelCatalog to modelPath.
// It streams the file, verifies sha256, and atomically renames it on success.
// If the file already exists with a matching sha256, it skips the download.
// It returns the path to the model file on success.
func Download(name, modelPath string) (string, error) {
	model, ok := ModelCatalog[name]
	if !ok {
		return "", fmt.Errorf("unknown model %q", name)
	}

	// Check if already downloaded with matching checksum.
	if fi, err := os.Stat(modelPath); err == nil && fi.Size() > 0 {
		if ok, err := verifySHA256(modelPath, model.SHA256); err != nil {
			return "", err
		} else if ok {
			return modelPath, nil
		}
		// Mismatch: delete and re-download.
		os.Remove(modelPath)
	}

	// Download.
	tmpPath := modelPath + ".tmp"
	if err := downloadFile(tmpPath, model.URL); err != nil {
		os.Remove(tmpPath)
		return "", fmt.Errorf("download failed: %w", err)
	}

	// Verify sha256.
	if ok, err := verifySHA256(tmpPath, model.SHA256); err != nil {
		os.Remove(tmpPath)
		return "", err
	} else if !ok {
		os.Remove(tmpPath)
		return "", fmt.Errorf("sha256 mismatch for %s", name)
	}

	// Atomic rename.
	if err := os.Rename(tmpPath, modelPath); err != nil {
		os.Remove(tmpPath)
		return "", fmt.Errorf("rename failed: %w", err)
	}

	return modelPath, nil
}

// verifySHA256 returns true if the file at path matches the expected hex-encoded sha256.
func verifySHA256(path, expected string) (bool, error) {
	f, err := os.Open(path)
	if err != nil {
		return false, err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return false, err
	}
	got := hex.EncodeToString(h.Sum(nil))
	return got == expected, nil
}

// downloadFile saves url to dst using a GET request with a reasonable timeout.
func downloadFile(dst, url string) error {
	const maxBytes = 500 << 20 // 500 MB limit
	client := &http.Client{}
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return err
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, resp.Status)
	}

	// Wrap with limited reader.
	body := io.LimitReader(resp.Body, maxBytes)
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, body)
	if err != nil {
		os.Remove(dst)
		return err
	}
	return out.Close()
}

// IsModelDownloaded returns true if the model file exists and passes sha256 check.
func IsModelDownloaded(name, modelPath string) bool {
	model, ok := ModelCatalog[name]
	if !ok {
		return false
	}
	ok, _ = verifySHA256(modelPath, model.SHA256)
	return ok
}
