// Package storage provides a client interface for object storage backends.
package storage

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"strings"
)

// Client abstracts file storage operations so backends can be swapped
// or mocked in tests.
type Client interface {
	// Upload stores a file and returns the storage file ID.
	Upload(ctx context.Context, filename string, reader io.Reader) (fid string, err error)
	// Download retrieves a file by its storage ID.
	Download(ctx context.Context, fid string) (io.ReadCloser, error)
	// Delete removes a file by its storage ID.
	Delete(ctx context.Context, fid string) error
}

// assignResponse is the JSON response from SeaweedFS POST /dir/assign.
type assignResponse struct {
	FID       string `json:"fid"`
	URL       string `json:"url"`
	PublicURL string `json:"publicUrl"`
	Count     int    `json:"count"`
	Error     string `json:"error"`
}

// SeaweedFSClient interacts with a SeaweedFS cluster over HTTP.
type SeaweedFSClient struct {
	masterURL  string
	httpClient *http.Client
}

// NewSeaweedFSClient creates a new SeaweedFS client pointing at the given master URL.
func NewSeaweedFSClient(masterURL string, httpClient *http.Client) *SeaweedFSClient {
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	// Trim trailing slashes for consistent URL construction.
	masterURL = strings.TrimRight(masterURL, "/")
	return &SeaweedFSClient{
		masterURL:  masterURL,
		httpClient: httpClient,
	}
}

// Upload assigns a file ID from the master, then uploads the file to the
// volume server. Returns the assigned fid on success.
func (c *SeaweedFSClient) Upload(ctx context.Context, filename string, reader io.Reader) (string, error) {
	// Step 1: Request a file ID from the master.
	assignReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.masterURL+"/dir/assign", nil)
	if err != nil {
		return "", fmt.Errorf("seaweedfs assign request: %w", err)
	}

	assignResp, err := c.httpClient.Do(assignReq)
	if err != nil {
		return "", fmt.Errorf("seaweedfs assign: %w", err)
	}
	defer func() { _ = assignResp.Body.Close() }()

	if assignResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(assignResp.Body)
		return "", fmt.Errorf("seaweedfs assign: unexpected status %d: %s", assignResp.StatusCode, string(body))
	}

	var assign assignResponse
	if err := json.NewDecoder(assignResp.Body).Decode(&assign); err != nil {
		return "", fmt.Errorf("seaweedfs assign decode: %w", err)
	}
	if assign.Error != "" {
		return "", fmt.Errorf("seaweedfs assign error: %s", assign.Error)
	}
	if assign.FID == "" || assign.URL == "" {
		return "", fmt.Errorf("seaweedfs assign: empty fid or url")
	}

	// Step 2: Upload the file content to the volume server.
	volumeURL := fmt.Sprintf("http://%s/%s", assign.URL, assign.FID)

	pr, pw := io.Pipe()
	writer := multipart.NewWriter(pw)

	// Write multipart form in a goroutine.
	errCh := make(chan error, 1)
	go func() {
		defer func() { _ = pw.Close() }()
		h := make(textproto.MIMEHeader)
		h.Set("Content-Disposition", fmt.Sprintf(`form-data; name="file"; filename="%s"`, filename))
		h.Set("Content-Type", "application/octet-stream")
		part, err := writer.CreatePart(h)
		if err != nil {
			errCh <- fmt.Errorf("create multipart part: %w", err)
			return
		}
		if _, err := io.Copy(part, reader); err != nil {
			errCh <- fmt.Errorf("copy to multipart: %w", err)
			return
		}
		errCh <- writer.Close()
	}()

	uploadReq, err := http.NewRequestWithContext(ctx, http.MethodPut, volumeURL, pr)
	if err != nil {
		return "", fmt.Errorf("seaweedfs upload request: %w", err)
	}
	uploadReq.Header.Set("Content-Type", writer.FormDataContentType())

	uploadResp, err := c.httpClient.Do(uploadReq)
	if err != nil {
		return "", fmt.Errorf("seaweedfs upload: %w", err)
	}
	defer func() { _ = uploadResp.Body.Close() }()

	// Wait for the multipart writer goroutine to complete.
	if writeErr := <-errCh; writeErr != nil {
		return "", fmt.Errorf("seaweedfs upload multipart write: %w", writeErr)
	}

	if uploadResp.StatusCode != http.StatusOK && uploadResp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(uploadResp.Body)
		return "", fmt.Errorf("seaweedfs upload: unexpected status %d: %s", uploadResp.StatusCode, string(body))
	}

	return assign.FID, nil
}

// Download retrieves the file content for the given fid.
// The caller is responsible for closing the returned ReadCloser.
func (c *SeaweedFSClient) Download(ctx context.Context, fid string) (io.ReadCloser, error) {
	// Lookup the volume URL for the fid.
	lookupURL := fmt.Sprintf("%s/dir/lookup?volumeId=%s", c.masterURL, volumeIDFromFID(fid))

	lookupReq, err := http.NewRequestWithContext(ctx, http.MethodGet, lookupURL, nil)
	if err != nil {
		return nil, fmt.Errorf("seaweedfs lookup request: %w", err)
	}

	lookupResp, err := c.httpClient.Do(lookupReq)
	if err != nil {
		return nil, fmt.Errorf("seaweedfs lookup: %w", err)
	}
	defer func() { _ = lookupResp.Body.Close() }()

	if lookupResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(lookupResp.Body)
		return nil, fmt.Errorf("seaweedfs lookup: unexpected status %d: %s", lookupResp.StatusCode, string(body))
	}

	var lookup lookupResponse
	if err := json.NewDecoder(lookupResp.Body).Decode(&lookup); err != nil {
		return nil, fmt.Errorf("seaweedfs lookup decode: %w", err)
	}
	if lookup.Error != "" {
		return nil, fmt.Errorf("seaweedfs lookup error: %s", lookup.Error)
	}
	if len(lookup.Locations) == 0 {
		return nil, fmt.Errorf("seaweedfs lookup: no locations for fid %s", fid)
	}

	downloadURL := fmt.Sprintf("http://%s/%s", lookup.Locations[0].URL, fid)
	dlReq, err := http.NewRequestWithContext(ctx, http.MethodGet, downloadURL, nil)
	if err != nil {
		return nil, fmt.Errorf("seaweedfs download request: %w", err)
	}

	dlResp, err := c.httpClient.Do(dlReq)
	if err != nil {
		return nil, fmt.Errorf("seaweedfs download: %w", err)
	}

	if dlResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(dlResp.Body)
		_ = dlResp.Body.Close()
		return nil, fmt.Errorf("seaweedfs download: unexpected status %d: %s", dlResp.StatusCode, string(body))
	}

	return dlResp.Body, nil
}

// Delete removes the file identified by fid from SeaweedFS.
func (c *SeaweedFSClient) Delete(ctx context.Context, fid string) error {
	// Lookup the volume URL for the fid.
	lookupURL := fmt.Sprintf("%s/dir/lookup?volumeId=%s", c.masterURL, volumeIDFromFID(fid))

	lookupReq, err := http.NewRequestWithContext(ctx, http.MethodGet, lookupURL, nil)
	if err != nil {
		return fmt.Errorf("seaweedfs lookup request: %w", err)
	}

	lookupResp, err := c.httpClient.Do(lookupReq)
	if err != nil {
		return fmt.Errorf("seaweedfs lookup: %w", err)
	}
	defer func() { _ = lookupResp.Body.Close() }()

	if lookupResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(lookupResp.Body)
		return fmt.Errorf("seaweedfs lookup: unexpected status %d: %s", lookupResp.StatusCode, string(body))
	}

	var lookup lookupResponse
	if err := json.NewDecoder(lookupResp.Body).Decode(&lookup); err != nil {
		return fmt.Errorf("seaweedfs lookup decode: %w", err)
	}
	if lookup.Error != "" {
		return fmt.Errorf("seaweedfs lookup error: %s", lookup.Error)
	}
	if len(lookup.Locations) == 0 {
		return fmt.Errorf("seaweedfs lookup: no locations for fid %s", fid)
	}

	deleteURL := fmt.Sprintf("http://%s/%s", lookup.Locations[0].URL, fid)
	delReq, err := http.NewRequestWithContext(ctx, http.MethodDelete, deleteURL, nil)
	if err != nil {
		return fmt.Errorf("seaweedfs delete request: %w", err)
	}

	delResp, err := c.httpClient.Do(delReq)
	if err != nil {
		return fmt.Errorf("seaweedfs delete: %w", err)
	}
	defer func() { _ = delResp.Body.Close() }()

	if delResp.StatusCode != http.StatusOK && delResp.StatusCode != http.StatusAccepted {
		body, _ := io.ReadAll(delResp.Body)
		return fmt.Errorf("seaweedfs delete: unexpected status %d: %s", delResp.StatusCode, string(body))
	}

	return nil
}

// lookupResponse is the JSON response from SeaweedFS GET /dir/lookup.
type lookupResponse struct {
	Locations []locationEntry `json:"locations"`
	Error     string          `json:"error"`
}

type locationEntry struct {
	URL       string `json:"url"`
	PublicURL string `json:"publicUrl"`
}

// volumeIDFromFID extracts the volume ID portion of a SeaweedFS file ID.
// A fid has the format "<volumeId>,<fileKey>" e.g. "3,01637037d6".
func volumeIDFromFID(fid string) string {
	if idx := strings.Index(fid, ","); idx >= 0 {
		return fid[:idx]
	}
	return fid
}
