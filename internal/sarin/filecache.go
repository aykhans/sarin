package sarin

import (
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"go.aykhans.me/sarin/internal/types"
)

// CachedFile holds the cached content and metadata of a file.
type CachedFile struct {
	Content  []byte
	Filename string
}

type FileCache struct {
	cache          sync.Map // map[string]*CachedFile
	requestTimeout time.Duration
}

func NewFileCache(requestTimeout time.Duration) *FileCache {
	return &FileCache{
		requestTimeout: requestTimeout,
	}
}

// GetOrLoad retrieves a file from cache or loads it using the provided source.
// The source can be a local file path or an HTTP/HTTPS URL.
// It can return the following errors:
//   - types.FileReadError
//   - types.HTTPFetchError
//   - types.HTTPStatusError
func (fc *FileCache) GetOrLoad(source string) (*CachedFile, error) {
	if val, ok := fc.cache.Load(source); ok {
		return val.(*CachedFile), nil
	}

	var (
		content  []byte
		filename string
		err      error
	)
	if strings.HasPrefix(source, "http://") || strings.HasPrefix(source, "https://") {
		content, filename, err = fc.fetchURL(source)
	} else {
		content, filename, err = fc.readLocalFile(source)
	}

	if err != nil {
		return nil, err
	}

	file := &CachedFile{Content: content, Filename: filename}

	// LoadOrStore handles race condition - if another goroutine
	// cached it first, we get theirs (no duplicate storage)
	actual, _ := fc.cache.LoadOrStore(source, file)
	return actual.(*CachedFile), nil
}

// readLocalFile reads a file from the local filesystem and returns its content and filename.
// It can return the following errors:
//   - types.FileReadError
func (fc *FileCache) readLocalFile(filePath string) ([]byte, string, error) {
	content, err := os.ReadFile(filePath) //nolint:gosec
	if err != nil {
		return nil, "", types.NewFileReadError(filePath, err)
	}
	return content, filepath.Base(filePath), nil
}

// fetchURL downloads file contents from an HTTP/HTTPS URL.
// It can return the following errors:
//   - types.HTTPFetchError
//   - types.HTTPStatusError
func (fc *FileCache) fetchURL(url string) ([]byte, string, error) {
	client := &http.Client{
		Timeout: fc.requestTimeout,
	}

	resp, err := client.Get(url)
	if err != nil {
		return nil, "", types.NewHTTPFetchError(url, err)
	}
	defer resp.Body.Close() //nolint:errcheck

	if resp.StatusCode != http.StatusOK {
		return nil, "", types.NewHTTPStatusError(url, resp.StatusCode, resp.Status)
	}

	content, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, "", types.NewHTTPFetchError(url, err)
	}

	// Extract filename from URL path
	filename := path.Base(url)
	if filename == "" || filename == "/" || filename == "." {
		filename = "downloaded_file"
	}

	// Remove query string from filename if present
	if idx := strings.Index(filename, "?"); idx != -1 {
		filename = filename[:idx]
	}

	return content, filename, nil
}
