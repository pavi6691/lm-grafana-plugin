package httpclient

import (
	"errors"
	"net/http"
	"strings"

	"github.com/grafana/grafana-logicmonitor-datasource-backend/pkg/constants"
)

func handleException(response *http.Response, err error) error {
	if err != nil {
		if strings.Contains(err.Error(), constants.NoSuchHostError) {
			err = errors.New(constants.InvalidCompanyName)
		} else if strings.Contains(err.Error(), constants.ConnectionRefused) || strings.Contains(err.Error(), constants.WriteTcpError) {
			err = errors.New(constants.NetworkError)
		} else if strings.Contains(err.Error(), constants.ConnectionTimeout) {
			err = errors.New(constants.ConnectionTimeoutError)
		}
		return err
	}

	if response.StatusCode == http.StatusServiceUnavailable ||
		response.StatusCode == http.StatusInternalServerError ||
		response.StatusCode == http.StatusBadRequest { // todo check exact error code when service is down
		return errors.New(constants.ServiceUnavailable)
	}

	if response.StatusCode == http.StatusTooManyRequests {
		return errors.New(constants.RateLimitErrMsg)
	}

	return nil
}
