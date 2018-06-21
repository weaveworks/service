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

	"github.com/prometheus/client_golang/prometheus"

	"github.com/weaveworks/common/logging"
	"github.com/weaveworks/service/common"
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

// ImportStatusResponse describes the status of a usage upload
type ImportStatusResponse struct {
	genericZuoraResponse
	ImportStatus string `json:"importStatus"` // can be: Pending, Processing, Completed, Canceled (or Cancelled ??? but docs say one `l`), Failed
	Message      string `json:"message"`
}

var usageImportHistogram = prometheus.NewHistogramVec(
	prometheus.HistogramOpts{
		Namespace: common.PrometheusNamespace,
		Subsystem: "zuora_client",
		Name:      "usage_import_duration_seconds",
		Help:      "Time taken for zuora to import usage data.",
	},
	[]string{"status"},
)

// UploadUsage uploads usage information to Zuora.
func (z *Zuora) UploadUsage(ctx context.Context, r io.Reader, id string) (string, error) {
	body := &bytes.Buffer{}
	// Create a new multipart writer. This is required, because this automates the setting of some funky headers.
	writer := multipart.NewWriter(body)
	// This creates a new "part". I.e. a section in the multi-part upload.
	// The word "file" is the name of the upload, and this is specified by zuora. The filename doesn't matter, but must not be null, and is limited to 50 chars!!
	part, err := writer.CreateFormFile("file", fmt.Sprintf("u-%.44s.csv", id))
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
	importStart := time.Now()
	err = z.Upload(
		ctx,
		postUsagePath,
		z.URL(postUsagePath),
		writer.FormDataContentType(),
		body,
		resp,
	)
	if err != nil {
		logging.With(ctx).Errorf("Usage upload failed! Upload body: %v", body)
		return "", err
	}
	if !resp.Success {
		logging.With(ctx).Errorf("Usage upload failed! Upload body: %v", body)
		return "", resp
	}

	logging.With(ctx).Infof("Import status url: %s", resp.CheckImportStatusURL)
	importStatusResp, err := z.WaitForImportFinished(ctx, resp.CheckImportStatusURL)
	if err != nil {
		return "", err
	}
	importDuration := time.Now().Sub(importStart)
	importStatus := importStatusResp.ImportStatus
	usageImportHistogram.WithLabelValues(importStatus).Observe(importDuration.Seconds())

	if importStatus != Completed {
		logging.With(ctx).Errorf("Usage import failed! Upload body: %v", body)
		return "", fmt.Errorf("Usage import did not succeed: %v - from %s", importStatusResp, resp.CheckImportStatusURL)
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
func (z *Zuora) GetUsageImportStatus(ctx context.Context, url string) (*ImportStatusResponse, error) {
	resp := ImportStatusResponse{}
	if err := z.Get(ctx, getImportStatusPath, url, resp); err != nil {
		return nil, err
	}
	if !resp.Success {
		return nil, &resp
	}
	return &resp, nil
}

// WaitForImportFinished waits for a usage import to complete and returns the status.
func (z *Zuora) WaitForImportFinished(ctx context.Context, statusURL string) (*ImportStatusResponse, error) {
	maxAttempts := 6
	var attempt int
	var resp *ImportStatusResponse
	for attempt = 0; attempt < maxAttempts; attempt++ {
		var statusCheckErr error
		logging.With(ctx).Infof("Checking usage import status")
		resp, statusCheckErr = z.GetUsageImportStatus(ctx, statusURL)
		if statusCheckErr == nil {
			importStatus := resp.ImportStatus
			if !(importStatus == "Pending" || importStatus == "Processing") {
				break
			}
		}
		sleepingTime := time.Duration(math.Pow(float64(2), float64(attempt))) * time.Second
		logging.With(ctx).Infof("Exponentially retrying in %v", sleepingTime)
		time.Sleep(sleepingTime)
	}
	if attempt < maxAttempts {
		return resp, nil
	}
	return nil, fmt.Errorf("Usage was not imported within %d retries", attempt)
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
