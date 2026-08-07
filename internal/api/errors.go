package api

import (
	"errors"

	beeperdesktopapi "github.com/beeper/desktop-api-go/v5"
)

// ErrorStatus extracts the HTTP status code from an SDK error chain, or 0
// when the error is not an API response error (e.g. a timeout). It lets
// callers log failure shape without the URL, which can embed chat IDs.
func ErrorStatus(err error) int {
	var apiErr *beeperdesktopapi.Error
	if errors.As(err, &apiErr) {
		return apiErr.StatusCode
	}
	return 0
}
