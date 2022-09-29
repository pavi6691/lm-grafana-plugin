package httpclient

import (
	"crypto/hmac"
	"crypto/sha256"
	b64 "encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/grafana/grafana-logicmonitor-datasource-backend/pkg/constants"
	"github.com/grafana/grafana-logicmonitor-datasource-backend/pkg/models"
	"github.com/grafana/grafana-plugin-sdk-go/backend/log"
)

func Get(pluginSettings *models.PluginSettings, authSettings *models.AuthSettings, requestURL string, logger log.Logger) (*http.Response, error) { //nolint:lll
	url := fmt.Sprintf(constants.RootURL, pluginSettings.Path) + requestURL
	client := &http.Client{} //nolint:exhaustivestruct

	httpRequest, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		logger.Error(" Error creating http request => ", err)

		return nil, err //nolint:wrapcheck
	}

	resourcePath := strings.ReplaceAll(httpRequest.URL.Path, constants.SantabaRestPath, "")

	// todo
	logger.Debug("The resource path is ", resourcePath)
	logger.Debug("The httpRequest.URL.Path is ", httpRequest.URL.Path)
	logger.Debug("The full path is ", requestURL)

	if pluginSettings.IsLMV1Enabled {
		httpRequest.Header.Add(constants.Authorization, getLMv1(pluginSettings.AccessID, authSettings.AccessKey, resourcePath)) //nolint:lll
	}

	if pluginSettings.IsBearerEnabled {
		httpRequest.Header.Add(constants.Authorization, buildBearerToken(authSettings))
	}

	httpRequest.Header.Add(constants.UserAgent, buildGrafanaUserAgent(pluginSettings))

	if resourcePath == constants.AutoCompleteNamesPath {
		httpRequest.Header.Add(constants.XVersion, constants.XVersionValue3)
	}

	//	//todo remove this
	// reqDump, err := httputil.DumpRequest(httpRequest, true)
	// if err != nil {
	// 	logger.Error(err.Error())
	// 	return nil, err
	// }

	// logger.Info("Hitting HTTP request with headers => ", string(reqDump), err)

	newResp, err := client.Do(httpRequest)
	if err != nil {
		logger.Error(" Error executing => "+url, err)
		if strings.Contains(err.Error(), constants.NoSuchHostError) {
			err = errors.New(constants.InvalidCompanyName)
		} else if strings.Contains(err.Error(), constants.ConnectionRefused) {
			err = errors.New(constants.NetworkError)
		}
		return nil, err
	}

	if newResp.StatusCode == http.StatusServiceUnavailable ||
		newResp.StatusCode == http.StatusInternalServerError ||
		newResp.StatusCode == http.StatusBadRequest {
		return nil, errors.New(constants.HostUnreachableErrMsg)
	}

	// todo high priority

	//	todo remove this
	// resDump, err := httputil.DumpResponse(newResp, true)
	// if err != nil {
	// 	logger.Error(err.Error())
	// 	return nil, err
	// }

	// logger.Info("HTTP response => "+string(resDump), err)

	return newResp, err //nolint:wrapcheck
}

func buildBearerToken(authSettings *models.AuthSettings) string {
	return "Bearer " + authSettings.BearerToken
}

func buildGrafanaUserAgent(pluginSettings *models.PluginSettings) string {
	return "LM-Grafana-" + pluginSettings.Path + ":" + pluginSettings.Version
}

func getLMv1(accessID, accessKey, resourcePath string) string {
	epoch := time.Now().UnixMilli()
	getEpoch := fmt.Sprintf("%s%d", http.MethodGet, epoch)
	data := getEpoch + resourcePath
	h := hmac.New(sha256.New, []byte(accessKey))
	h.Write([]byte(data))
	sha := hex.EncodeToString(h.Sum(nil))
	auth := constants.LMv1 + " " + accessID + ":" + b64.URLEncoding.EncodeToString([]byte(sha)) + fmt.Sprintf("%s%d", ":", epoch) //nolint:lll

	return auth
}
