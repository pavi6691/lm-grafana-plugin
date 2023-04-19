package cache

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/ReneKroon/ttlcache"
	"github.com/grafana/grafana-logicmonitor-datasource-backend/pkg/constants"
	httpclient "github.com/grafana/grafana-logicmonitor-datasource-backend/pkg/httpclient"
	"github.com/grafana/grafana-logicmonitor-datasource-backend/pkg/models"
	utils "github.com/grafana/grafana-logicmonitor-datasource-backend/pkg/utils"
	"github.com/grafana/grafana-plugin-sdk-go/backend"
)

// Stores mapping of host data source id against ket host and datasource. caching this mapping avoids multiple API call for when host variable is changed
var hostDsAndHdsMapping = ttlcache.NewCache() //nolint:gochecknoglobals

func get(key string) (interface{}, bool) {
	if v, ok := hostDsAndHdsMapping.Get(key); ok {
		return v, true
	}
	return nil, false
}

func add(key string, value interface{}) {
	hostDsAndHdsMapping.SetWithTTL(key, value, time.Duration(constants.InterpolateDataCacheTTLMinutes*60)*time.Second)
}

func InterpolateHostDataSourceDetails(santabaClient httpclient.SantabaClient, queryModel models.QueryModel,
	response backend.DataResponse) (models.QueryModel, backend.DataResponse) {
	hdsSelected, present := get(fmt.Sprintf("%s-%d", queryModel.HostSelected.Value, queryModel.DataSourceSelected.Ds))
	if present {
		queryModel.HdsSelected = hdsSelected.(int64)
		return queryModel, response
	}
	requestURL := utils.BuildURLReplacingQueryParams(constants.HostDataSourceReq, &queryModel, 0, 0, models.MetaData{})
	if requestURL == "" {
		santabaClient.Logger.Error(constants.URLConfigurationErrMsg)
		return queryModel, response
	}
	var respByte []byte
	respByte, response.Error = santabaClient.Get(requestURL, constants.HostDataSourceReq)
	if response.Error != nil {
		santabaClient.Logger.Error("Error from server => ", response.Error)
		return queryModel, response
	}
	var hdsReponse models.HostDataSource
	response.Error = json.Unmarshal(respByte, &hdsReponse)
	if response.Error != nil {
		santabaClient.Logger.Error(constants.ErrorUnmarshallingErrorData+"hdsReponse =>", response.Error.Error())
		return queryModel, response
	}
	if hdsReponse.Total == 1 {
		queryModel.HdsSelected = hdsReponse.Items[0].Id
		add(fmt.Sprintf("%s-%d", queryModel.HostSelected.Value, queryModel.DataSourceSelected.Ds), queryModel.HdsSelected)
	} else if hdsReponse.Total > 1 {
		response.Error = errors.New(constants.MoreThanOneHostDataSources + queryModel.DataSourceSelected.Label)
		return queryModel, response
	} else {
		response.Error = fmt.Errorf(constants.HostHasNoMatchingDataSource, queryModel.DataSourceSelected.Label)
		return queryModel, response
	}
	return queryModel, response
}

func InterpolateHostDetails(santabaClient httpclient.SantabaClient, queryModel models.QueryModel,
	response backend.DataResponse) (models.QueryModel, backend.DataResponse) {
	hostId, present := get(queryModel.HostSelected.Label)
	if present {
		queryModel.HostSelected.Value = hostId.(string)
		return queryModel, response
	}
	requestURL := utils.BuildURLReplacingQueryParams(constants.AutoCompleteHostReq, &queryModel, 0, 0, models.MetaData{})
	if requestURL == "" {
		santabaClient.Logger.Error(constants.URLConfigurationErrMsg)
		return queryModel, response
	}
	var respByte []byte
	santabaClient.Logger.Warn("Calling to interpolate host", requestURL)
	respByte, response.Error = santabaClient.Get(requestURL, constants.AutoCompleteHostReq)
	if response.Error != nil {
		santabaClient.Logger.Error("Error from server => ", response.Error)
		return queryModel, response
	}
	var autoCompleteHosts models.AutoCompleteHosts
	response.Error = json.Unmarshal(respByte, &autoCompleteHosts)
	if response.Error != nil {
		santabaClient.Logger.Error(constants.ErrorUnmarshallingErrorData+"hdsReponse =>", response.Error.Error())
		return queryModel, response
	}
	if len(autoCompleteHosts.Items) > 0 {
		queryModel.HostSelected.Value = strings.Split(autoCompleteHosts.Items[0], ":")[0]
		add(queryModel.HostSelected.Label, queryModel.HostSelected.Value)
	} else {
		response.Error = fmt.Errorf(constants.NoHostFoundForGivenGlobPattern, queryModel.HostSelected.Label)
		return queryModel, response
	}
	return queryModel, response
}
