package httpclient

import (
	"crypto/hmac"
	"crypto/sha256"
	b64 "encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"github.com/grafana/grafana-plugin-sdk-go/backend/log"
	"io/ioutil"
	"net/http"
	"net/http/httputil"
	"strings"
	"time"

	"github.com/grafana/grafana-logicmonitor-datasource-backend/pkg/constants"
	"github.com/grafana/grafana-logicmonitor-datasource-backend/pkg/models"
)

type SantabaResource interface {
	Get(requestURL string, request string) ([]byte, error)
}

type SantabaClient struct {
	PluginSettings *models.PluginSettings
	AuthSettings   *models.AuthSettings
	Client         *http.Client
	Logger         log.Logger
}

func (santabaClient SantabaClient) Get(requestURL string, request string) ([]byte, error) { //nolint:lll
	url := fmt.Sprintf(constants.RootURL, santabaClient.PluginSettings.Path) + requestURL
	httpRequest, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		santabaClient.Logger.Error(constants.ErrorCreatingHttpRequest, err)

		return nil, err //nolint:wrapcheck
	}

	resourcePath := strings.ReplaceAll(httpRequest.URL.Path, constants.SantabaRestPath, "")

	// todo
	santabaClient.Logger.Debug("The resource path is ", resourcePath)
	santabaClient.Logger.Debug("The httpRequest.URL.Path is ", httpRequest.URL.Path)
	santabaClient.Logger.Debug("The full path is ", requestURL)

	if santabaClient.PluginSettings.IsLMV1Enabled {
		httpRequest.Header.Add(constants.Authorization, getLMv1(santabaClient.PluginSettings.AccessID, santabaClient.AuthSettings.AccessKey, resourcePath)) //nolint:lll
	}

	if santabaClient.PluginSettings.IsBearerEnabled {
		httpRequest.Header.Add(constants.Authorization, buildBearerToken(santabaClient.AuthSettings))
	}

	httpRequest.Header.Add(constants.UserAgent, buildGrafanaUserAgent(santabaClient.PluginSettings))

	if resourcePath == constants.AutoCompleteNamesPath || request == constants.HostDataSourceReq {
		httpRequest.Header.Add(constants.XVersion, constants.XVersionValue3)
	}

	//	//todo remove this
	reqDump, err := httputil.DumpRequest(httpRequest, true)
	if err != nil {
		santabaClient.Logger.Error(err.Error())
		return nil, err
	}

	santabaClient.Logger.Debug("Hitting HTTP request with headers => ", string(reqDump), err)

	newResp, err := santabaClient.Client.Do(httpRequest)
	var respByte []byte
	if err != nil {
		santabaClient.Logger.Error(constants.HttpClientErrorMakingRequest, err)
	} else {
		defer newResp.Body.Close()
		respByte, err = ioutil.ReadAll(newResp.Body)
		if err != nil {
			santabaClient.Logger.Error(constants.ErrorReadingResponseBody, err)
			return nil, errors.New(constants.ErrorReadingResponseBody)
		}
	}
	err = handleException(newResp, respByte, err)
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
