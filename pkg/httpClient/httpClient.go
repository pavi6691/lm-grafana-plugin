package httpclient

import (
	"crypto/hmac"
	"crypto/sha256"
	b64 "encoding/base64"
	"encoding/hex"
	"fmt"
	"net/http"
	"time"

	"github.com/grafana/grafana-logicmonitor-datasource-backend/pkg/constants"
	"github.com/grafana/grafana-plugin-sdk-go/backend/log"
)

func Get(accessId, accessKey, Bearer_token, resourcePath, fullPath, host string, version string) (*http.Response, error) {
	url := fmt.Sprintf(constants.RootUrl, host) + fullPath
	client := &http.Client{}
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		log.DefaultLogger.Info(" Error creating http request => ", err)
	}
	if len(Bearer_token) > 0 {
		req.Header.Add("Authorization", "Bearer "+Bearer_token)
	} else {
		req.Header.Add("Authorization", getLMv1(accessId, accessKey, "/"+resourcePath))
	}
	req.Header.Add("User-Agent", "LM-Grafana-"+host+":"+version)

	if resourcePath == "autocomplete/names" {
		req.Header.Add("x-version", "3")
	}
	resp, err := client.Do(req)
	if err != nil {
		log.DefaultLogger.Info(" Error executing => "+url, err)
	}
	return resp, err
}

func getLMv1(accessId, accessKey, resourcePath string) string {
	epoch := time.Now().UnixMilli()
	getEpoch := fmt.Sprintf("%s%d", "GET", epoch)
	data := getEpoch + resourcePath
	h := hmac.New(sha256.New, []byte(accessKey))
	h.Write([]byte(data))
	sha := hex.EncodeToString(h.Sum(nil))
	auth := "LMv1 " + accessId + ":" + b64.URLEncoding.EncodeToString([]byte(sha)) + fmt.Sprintf("%s%d", ":", epoch)
	return auth
}
