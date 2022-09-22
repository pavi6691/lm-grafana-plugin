package main

import (
	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"os"

	plugin "github.com/grafana/grafana-logicmonitor-datasource-backend/pkg/datasource"
	"github.com/grafana/grafana-plugin-sdk-go/backend/datasource"
	"github.com/grafana/grafana-plugin-sdk-go/backend/log"
)

const LOGICMONITOR_PLUGIN_ID = "logicmonitor-datasource"

func main() {
	// Start listening to requests sent from Grafana. This call is blocking so
	// it won't finish until Grafana shuts down the process or the plugin choose
	// to exit by itself using os.Exit. Manage automatically manages life cycle
	// of datasource instances. It accepts datasource instance factory as first
	// argument. This factory will be automatically called on incoming request
	// from Grafana to create different instances of LogicmonitorDataSource (per datasource
	// ID). When datasource configuration changed Dispose method will be called and
	// new datasource instance created using LogicmonitorBackendDataSource factory.
	//todo plugin id and docs
	backend.SetupPluginEnvironment(LOGICMONITOR_PLUGIN_ID)
	pluginLogger := log.New()
	pluginLogger.Debug("Starting Zabbix datasource")
	if err := datasource.Manage(LOGICMONITOR_PLUGIN_ID, plugin.LogicmonitorBackendDataSource, datasource.ManageOpts{}); err != nil {
		log.DefaultLogger.Error(err.Error())
		pluginLogger.Error("Error starting Logicmonitor datasource", "error", err.Error())
		os.Exit(1)
	}
}
