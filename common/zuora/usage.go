package zuora

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"mime/multipart"
	"regexp"

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
	CheckImportStatus string `json:"checkImportStatus"`
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
func (z *Zuora) UploadUsage(ctx context.Context, r io.Reader) (string, error) {
	body := &bytes.Buffer{}
	// Create a new multipart writer. This is required, because this automates the setting of some funky headers.
	writer := multipart.NewWriter(body)
	// This creates a new "part". I.e. a section in the multi-part upload.
	// The word "file" is the name of the upload, and this is specified by zuora. The filename doesn't matter, but must not be null!!
	part, err := writer.CreateFormFile("file", "billing-uploader.csv")
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

	logging.With(ctx).Infof("Response from Zuora: %v", resp.CheckImportStatus)
	importID, err := extractUsageImportID(resp.CheckImportStatus)
	if err != nil {
		return "", err
	}
	return importID, nil
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
func (z *Zuora) GetUsageImportStatus(ctx context.Context, importID string) (string, error) {
	resp := &importStatusResponse{}
	if err := z.Get(ctx, getUsagePath, z.URL(getImportStatusPath, importID), resp); err != nil {
		return "", err
	}
	if !resp.Success {
		return "", resp
	}
	return resp.ImportStatus, nil
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
