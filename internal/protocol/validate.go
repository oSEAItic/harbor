package protocol

import (
	"fmt"
)

// ValidateResponse checks that a connector response conforms to the protocol.
func ValidateResponse(resp *Response) error {
	if resp == nil {
		return fmt.Errorf("response is nil")
	}

	if resp.Meta.Source == "" {
		return fmt.Errorf("meta.source is required")
	}
	if resp.Meta.RequestID == "" {
		return fmt.Errorf("meta.request_id is required")
	}
	if resp.Meta.FetchedAt.IsZero() {
		return fmt.Errorf("meta.fetched_at is required")
	}
	if resp.Meta.Schema == "" {
		return fmt.Errorf("meta.schema is required")
	}
	if resp.Meta.ConnectorVersion == "" {
		return fmt.Errorf("meta.connector_version is required")
	}

	// data must be present (even if empty array)
	if resp.Data == nil {
		return fmt.Errorf("data field is required")
	}

	return nil
}

// NewErrorResponse creates a response containing only errors.
func NewErrorResponse(source string, requestID string, code ErrorCode, message string) *Response {
	return &Response{
		Data: []byte("[]"),
		Meta: Meta{
			Source:    source,
			RequestID: requestID,
		},
		Raw: nil,
		Errors: []ErrorDetail{
			{Code: code, Message: message},
		},
	}
}
