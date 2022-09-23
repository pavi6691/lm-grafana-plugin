package httpclient

import (
	"crypto/hmac"
	"crypto/sha256"
	b64 "encoding/base64"
	"encoding/hex"
	"fmt"
	"github.com/grafana/grafana-logicmonitor-datasource-backend/pkg/constants"
	"github.com/grafana/grafana-plugin-sdk-go/backend/log"
	"net/http/httputil"

	//nolint:typecheck
	"github.com/grafana/grafana-logicmonitor-datasource-backend/pkg/models"
	"net/http"
	"time"
)

func Get(pluginSettings *models.PluginSettings, authSettings *models.AuthSettings, resourcePath string, fullPath string, logger log.Logger) (*http.Response, error) { //nolint:lll
	url := fmt.Sprintf(constants.RootUrl, pluginSettings.Path) + fullPath
	client := &http.Client{} //nolint:exhaustivestruct

	logger.Info("Hitting HTTP request => " + url)

	httpRequest, err := http.NewRequest(constants.Get, url, nil)
	if err != nil {
		logger.Info(" Error creating http request => ", err)
	}

	if pluginSettings.IsLMV1Enabled {
		httpRequest.Header.Add(constants.Authorization, getLMv1(pluginSettings.AccessID, authSettings.AccessKey, "/"+resourcePath)) //nolint:lll
	}

	if pluginSettings.IsBearerEnabled {
		httpRequest.Header.Add(constants.Authorization, buildBearerToken(authSettings))
	}

	httpRequest.Header.Add(constants.UserAgent, buildGrafanaUserAgent(pluginSettings))

	if resourcePath == constants.AutoCompleteNames {
		httpRequest.Header.Add(constants.XVersion, constants.XVersionValue3)
	}

	//todo remove this
	reqDump, err := httputil.DumpRequest(httpRequest, true)
	if err != nil {
		logger.Error(err.Error())
	}

	logger.Info("Hitting HTTP request with headers => "+string(reqDump), err)

	resp, err := client.Do(httpRequest)
	if err != nil {
		logger.Info(" Error executing => "+url, err)
	}
	defer resp.Body.Close()

	//todo remove this
	resDump, err := httputil.DumpResponse(resp, true)
	if err != nil {
		logger.Error(err.Error())
	}

	logger.Info("HTTP response => "+string(resDump), err)

	return resp, err
}

func buildBearerToken(authSettings *models.AuthSettings) string {
	return "Bearer " + authSettings.BearerToken
}

func buildGrafanaUserAgent(pluginSettings *models.PluginSettings) string {
	return "LM-Grafana-" + pluginSettings.Path + ":" + pluginSettings.Version
}

func getLMv1(accessID, accessKey, resourcePath string) string {
	epoch := time.Now().UnixMilli()
	getEpoch := fmt.Sprintf("%s%d", "GET", epoch)
	data := getEpoch + resourcePath
	h := hmac.New(sha256.New, []byte(accessKey))
	h.Write([]byte(data))
	sha := hex.EncodeToString(h.Sum(nil))
	auth := constants.LMv1 + " " + accessID + ":" + b64.URLEncoding.EncodeToString([]byte(sha)) + fmt.Sprintf("%s%d", ":", epoch) //nolint:lll

	return auth
}
