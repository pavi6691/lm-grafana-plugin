package logicmonitor

import (
	"encoding/json"
	"sync"

	"github.com/grafana/grafana-plugin-sdk-go/backend/log"

	"github.com/grafana/grafana-logicmonitor-datasource-backend/pkg/constants"
	"github.com/grafana/grafana-logicmonitor-datasource-backend/pkg/httpclient"
	"github.com/grafana/grafana-logicmonitor-datasource-backend/pkg/models"
)

type Job struct {
	JobId    int
	TimeFrom int64
	TimeTo   int64
}

// Gets fresh data by calling rest API
func CallDataAPI(wg *sync.WaitGroup, jobs chan Job, results map[int]*models.MultiInstanceRawData, queryModel *models.QueryModel, pluginSettings *models.PluginSettings,
	authSettings *models.AuthSettings, metaData models.MetaData, AppendAndDelete bool, logger log.Logger) {
	defer wg.Done()
	for job := range jobs {

		var rawData models.MultiInstanceRawData //nolint:exhaustivestruct
		rawData.AppendAndDelete = AppendAndDelete

		fullPath := BuildURLReplacingQueryParams(constants.RawDataMultiInstanceReq, queryModel, job.TimeFrom, job.TimeTo, metaData)

		logger.Debug("Calling API  => ", pluginSettings.Path, fullPath)
		//todo remove the loggers

		respByte, err := httpclient.Get(pluginSettings, authSettings, fullPath, constants.RawDataMultiInstanceReq, logger)
		if err != nil {
			rawData.Error = err.Error()
			logger.Error("Error from server => ", err)
		} else {
			err = json.Unmarshal(respByte, &rawData)
			if err != nil {
				rawData.Error = err.Error()
				logger.Error(constants.ErrorUnmarshallingErrorData+"raw-data => ", err)
			}
		}
		logger.Debug("1", job.JobId)
		results[job.JobId] = &rawData
		logger.Debug("2", job.JobId)
	}
}
