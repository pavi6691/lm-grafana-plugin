package httpclient

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/tls"
	b64 "encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httputil"
	"strings"
	"time"

	"github.com/grafana/grafana-logicmonitor-datasource-backend/pkg/constants"
	"github.com/grafana/grafana-logicmonitor-datasource-backend/pkg/models"
	"github.com/grafana/grafana-plugin-sdk-go/backend/log"
)

func Get(pluginSettings *models.PluginSettings, authSettings *models.AuthSettings, requestURL string, req string, logger log.Logger) ([]byte, error) { //nolint:lll
	url := fmt.Sprintf(constants.RootURL, pluginSettings.Path) + requestURL
	client := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: pluginSettings.SkipTLSVarify,
			},
		},
	}

	httpRequest, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		logger.Error(constants.ErrorCreatingHttpRequest, err)

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

	if resourcePath == constants.AutoCompleteNamesPath || req == constants.HostDataSourceReq {
		httpRequest.Header.Add(constants.XVersion, constants.XVersionValue3)
	}

	//	//todo remove this
	reqDump, err := httputil.DumpRequest(httpRequest, true)
	if err != nil {
		logger.Error(err.Error())
		return nil, err
	}

	logger.Debug("Hitting HTTP request with headers => ", string(reqDump), err)

	newResp, err := client.Do(httpRequest)
	var respByte []byte
	if err != nil {
		logger.Error(constants.HttpClientErrorMakingRequest, err)
	} else {
		defer newResp.Body.Close()
		respByte, err = ioutil.ReadAll(newResp.Body)
		if err != nil {
			logger.Error(constants.ErrorReadingResponseBody, err)
			return nil, errors.New(constants.ErrorReadingResponseBody)
		}
	}
	err = handleException(newResp, err)
	if err != nil {
		return nil, err
	}

	// todo high priority

	//	todo remove this
	// resDump, err := httputil.DumpResponse(newResp, true)
	// if err != nil {
	// 	logger.Error(err.Error())
	// 	return nil, err
	// }

	// logger.Info("HTTP response => "+string(resDump), err)

	return respByte, err //nolint:wrapcheck
}

func buildBearerToken(authSettings *models.AuthSettings) string {
	return constants.BearerTokenPrefix + authSettings.BearerToken
}

func buildGrafanaUserAgent(pluginSettings *models.PluginSettings) string {
	return fmt.Sprintf(constants.GrafanaUserAgent, pluginSettings.Path, pluginSettings.Version)
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
