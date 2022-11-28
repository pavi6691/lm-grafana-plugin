package httpclient

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/grafana/grafana-logicmonitor-datasource-backend/pkg/constants"
)

func handleException(response *http.Response, respByte []byte, err error) error {
	if err != nil {
		if strings.Contains(err.Error(), constants.NoSuchHostError) {
			err = errors.New(constants.InvalidCompanyName)
		} else if strings.Contains(err.Error(), constants.ConnectionRefused) || strings.Contains(err.Error(), constants.WriteTcpError) {
			err = errors.New(constants.NetworkError)
		} else if strings.Contains(err.Error(), constants.ConnectionTimeout) {
			err = errors.New(constants.ConnectionTimeoutError)
		} else if strings.Contains(err.Error(), constants.ConnectionUnReachable) {
			err = errors.New(constants.ConnectionTimeoutError)
		} else {
			err = errors.New(constants.HttpClientErrorMakingRequest)
		}
		return err
	}

	if response.StatusCode == http.StatusServiceUnavailable { // todo check exact error code when service is down
		return errors.New(constants.ServiceUnavailable)
	}

	if response.StatusCode == http.StatusTooManyRequests {
		return errors.New(constants.RateLimitErrMsg)
	}

	if response.StatusCode != http.StatusOK {
		return errors.New(fmt.Sprintf(constants.HttpNotOk, response.StatusCode, string(respByte)))
	}

	return nil
}
