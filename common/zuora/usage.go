package zuora

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"math"
	"mime/multipart"
	"regexp"
	"time"

	"github.com/weaveworks/common/logging"
)

const (
	postUsagePath       = "usage"
	getUsagePath        = "usage/accounts/%s"
	getImportStatusPath = "usage/%s/status"
)

// Usage represents a Zuora usage row.
type Usage struct {
	AccountID string  `json:"accountId"`
	StartDate string  `json:"startDateTime"`
	UnitType  string  `json:"unitOfMeasure"`
	Quantity  float64 `json:"quantity"`
	Status    string  `json:"status"` // 'Importing'|'Pending'|'Processed'
}

type postUsageResponse struct {
	genericZuoraResponse
	CheckImportStatusURL string `json:"checkImportStatus"`
}

type getUsageResponse struct {
	Success bool    `json:"success"`
	Usages  []Usage `json:"usage"`
}

type importStatusResponse struct {
	genericZuoraResponse
	ImportStatus string `json:"importStatus"` // can be: Pending, Processing, Completed, Canceled (or Cancelled ??? but docs say one `l`), Failed
}

// UploadUsage uploads usage information to Zuora.
func (z *Zuora) UploadUsage(ctx context.Context, r io.Reader, id string) (string, error) {
	body := &bytes.Buffer{}
	// Create a new multipart writer. This is required, because this automates the setting of some funky headers.
	writer := multipart.NewWriter(body)
	// This creates a new "part". I.e. a section in the multi-part upload.
	// The word "file" is the name of the upload, and this is specified by zuora. The filename doesn't matter, but must not be null!!
	part, err := writer.CreateFormFile("file", fmt.Sprintf("upload-%s.csv", id))
	if err != nil {
		return "", err
	}
	// Copy the report CSV to the part body
	_, err = io.Copy(part, r)
	if err != nil {
		return "", err
	}
	err = writer.Close()
	if err != nil {
		return "", err
	}

	resp := &postUsageResponse{}
	err = z.Upload(
		ctx,
		postUsagePath,
		z.URL(postUsagePath),
		writer.FormDataContentType(),
		body,
		resp,
	)
	if err != nil {
		return "", err
	}
	if !resp.Success {
		return "", resp
	}

	logging.With(ctx).Infof("Import status url: %s", resp.CheckImportStatusURL)
	if err != nil {
		return "", err
	}
	importStatus, err := z.WaitForImportFinished(ctx, resp.CheckImportStatusURL)
	if err != nil {
		return "", err
	}
	if importStatus != "Completed" {
		return "", fmt.Errorf("Usage import did not succeed: %s - see %s", importStatus, resp.CheckImportStatusURL)
	}
	return extractUsageImportID(resp.CheckImportStatusURL)
}

// GetUsage retrieves paginated usages of given organization.
func (z *Zuora) GetUsage(ctx context.Context, zuoraAccountNumber, page, pageSize string) ([]Usage, error) {
	if zuoraAccountNumber == "" {
		return nil, ErrInvalidAccountNumber
	}
	url := z.URL(getUsagePath, zuoraAccountNumber)
	url = url + "?" + pagingParams(page, pageSize).Encode()
	resp := &getUsageResponse{}
	if err := z.Get(ctx, getUsagePath, url, resp); err != nil {
		return nil, err
	}
	if !resp.Success {
		return nil, ErrNotFound
	}
	return resp.Usages, nil
}

// GetUsageImportStatus returns the Zuora status of a given usage.
func (z *Zuora) GetUsageImportStatus(ctx context.Context, url string) (string, error) {
	resp := &importStatusResponse{}
	if err := z.Get(ctx, getImportStatusPath, url, resp); err != nil {
		return "", err
	}
	if !resp.Success {
		return "", resp
	}
	return resp.ImportStatus, nil
}

// WaitForImportFinished waits for a usage import to complete and returns the status.
func (z *Zuora) WaitForImportFinished(ctx context.Context, statusURL string) (string, error) {
	maxAttempts := 6
	var attempt int
	var importStatus string
	for attempt = 0; attempt < maxAttempts; attempt++ {
		logging.With(ctx).Infof("Checking usage import status")
		importStatus, statusCheckErr := z.GetUsageImportStatus(ctx, statusURL)
		if statusCheckErr == nil && !(importStatus == "Pending" || importStatus == "Processing") {
			break
		}
		sleepingTime := time.Duration(math.Pow(float64(2), float64(attempt))) * time.Second
		logging.With(ctx).Infof("Exponentially retrying in %v", sleepingTime)
		time.Sleep(sleepingTime)
	}
	if attempt < maxAttempts {
		return importStatus, nil
	}
	return "", fmt.Errorf("Usage was not imported within %d retries", attempt)
}

func extractUsageImportID(path string) (string, error) {
	re := regexp.MustCompile("/v1/usage/([a-z0-9]*)/status")
	match := re.FindStringSubmatch(path)
	// match should return 2 elements because the left most match is the entire string.
	// see https://golang.org/pkg/regexp/#Regexp.FindStringSubmatch
	if len(match) != 2 {
		return "", fmt.Errorf("Could not parse usage import status id path: %v", path)
	}
	return match[1], nil
}
