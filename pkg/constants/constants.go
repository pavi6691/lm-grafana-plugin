package constants

const (
	RootUrl               = "https://%s.logicmonitor.com/santaba/rest/"
	SantabaRestPath       = "/santaba/rest"
	AutoCompleteNamesPath = "/autocomplete/names"
)

const (
	AccessKey   = "accessKey"
	BearerToken = "bearer_token"
)

const (
	NoData          = "No Data"
	Response        = "response"
	Time            = "time"
	RequestNotValid = "Request not valid"
)

const (
	Authorization  = "Authorization"
	LMv1           = "LMv1"
	XVersion       = "x-version"
	XVersionValue3 = "3"
	UserAgent      = "User-Agent"
)

const (
	NoCompanyNameEnteredErrMsg      = "Company name not entered"
	NoAuthenticationErrMsg          = "Please Authenticate to use the plugin"
	BearerTokenEmptyErrMsg          = "Please enter bearer token"
	AccessKeyEmptyErrMsg            = "Please enter Access Key"
	AccessIDEmptyErrMsg             = "Please enter AccessId"
	HealthAPIErrMsg                 = "Issue with Health API call to Logicmonitor"
	HealthAPIURLErrMsg              = "Issue with Health API URL configuration"
	HostUnreachableErrMsg           = "Host not reachable / invalid company name configured"
	APIErrMsg                       = "API Failed with status code = "
	URLConfigurationErrMsg          = "URL configuration missing in Backend"
	DataNotPresentCacheErrMsg       = "Data not present in Cache"
	DataNotPresentEditorCacheErrMsg = "Data not present in Editor Cache"
	InvalidTokenErrMsg              = "Invalid Token for Company or " //nolint:gosec
	AuthSuccessMsg                  = "Authentication Success"
)

// These constants are from PathEndpoints.ts
const (
	AllInstanceReq          = "AllInstanceReq"
	AllHostReq              = "AllHostReq"
	AutoCompleteInstanceReq = "AutoCompleteInstanceReq"
	AutoCompleteHostReq     = "AutoCompleteHostReq"
	ServiceOrDeviceGroupReq = "ServiceOrDeviceGroupReq"
	AutoCompleteGroupReq    = "AutoCompleteGroupReq"
	DataSourceReq           = "DataSourceReq"
	DataPointReq            = "DataPointReq"

	// Below constants are not used in Frontend
	RawDataSingleInstaceReq = "RawDataReq"
	RawDataMultiInstanceReq = "RawDataMultiInstanceReq"
	HealthCheckReq          = "HealthCheckReq"
)

const (
	// AutoCompleteGroupUrl = Groups, gets both device and service
	AutoCompleteGroupURL = "autocomplete/names?queryToken=display&filterFlag=ImmediateChild&size=10&_=%d&type=hostChain&query=%s&parentsFilters=[]" //nolint:lll

	// GroupExtraFilters = Groups, gets either service / devices
	GroupExtraFilters = `{"AND":[{"OR":[{"name":"groupType","value":"%s","op":":"},{"name":"id","value":1,"op":":"}]},
								{"name":"userPermission","value":"write","op":":"}, {"OR":[{"name":"fullPath","value":"%s","op":"~"},
								{"name":"name","value":"%s","op":"~"}]}]}`
	ServiceOrDeviceGroupURL = "device/groups?fields=id,fullPath,name&sort=fullPath&size=10&_=%d&extraFilters="

	// HostParentFilters  = Devices
	HostParentFilters    = `[{"filter":"%s","exclude":false,"token":"fullname","matchFilterAsGlob":true}]`
	AutoCompleteHostsURL = `autocomplete/names?queryToken=display&needIdPrefix=true&size=10&_=%d&type=hostChain&query=%s&parentsFilters=` //nolint:lll

	DataSourceURL = `device/devices/%s/devicedatasources?format=json&fields=id,dataSourceDisplayName,dataSourceId,instanceNumber&size=-1&filter=instanceNumber>:1` //nolint:lll

	InstanceParentFilters = `[{"filter":"%s","exclude":false,"token":"fullname","matchFilterAsGlob":true},{"filter":"%s","exclude":false,"token":"display","matchFilterAsGlob":true},{"filter":"%s","exclude":false,"token":"display","matchFilterAsGlob":false}]` //nolint:lll

	AutoCompleteInstanceURL = `autocomplete/names?queryToken=shortname&needIdPrefix=true&size=10&_=%d&type=hostDsChain&query=%s&parentsFilters=` //nolint:lll

	// DataPointURL DataPoints
	DataPointURL = "setting/datasources/%d?format=json&fields=dataPoints,collectInterval"

	HealthCheckURL = "device/devices?size=1"

	RawDataSingleInstanceURL = "device/devices/%s/devicedatasources/%d/instances/%s/data?start=%d&end=%d"

	RawDataMultiInstanceURL = "device/devices/%s/devicedatasources/%d/data?start=%d&end=%d"

	// -------- If autocomplete is disabled below APIs are used
	// AllHostURL = Get All Hosts
	AllHostURL = "device/devices?format=json&fields=id,displayName&size=-1"

	// AllInstanceURL = Get All Instances by hostId and Host Datasource Id
	AllInstanceURL = "device/devices/%s/devicedatasources/%d/instances?format=json&fields=id,name&size=-1"
)

const (
	InstantAndDpDelim byte = '-'
)
