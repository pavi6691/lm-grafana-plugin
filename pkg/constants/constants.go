package constants

const (
	RootURL               = "https://%s.logicmonitor.com/santaba/rest/"
	SantabaRestPath       = "/santaba/rest"
	AutoCompleteNamesPath = "/autocomplete/names"
)

const (
	AccessKey   = "accessKey"
	BearerToken = "bearer_token"
)

const (
	NoData             = "No Data"
	ResponseStr        = "response"
	TimeStr            = "time"
	RequestNotValidStr = "Request not valid"
)

const (
	Authorization     = "Authorization"
	LMv1              = "LMv1"
	XVersion          = "x-version"
	XVersionValue3    = "3"
	UserAgent         = "User-Agent"
	BearerTokenPrefix = "Bearer "
	GrafanaUserAgent  = "LM-Grafana-%s:%s"
)

const (
	Regex  = "Regex"
	Select = "Select"
)

const (
	NoCompanyNameEnteredErrMsg        = "Company name not entered"
	NoAuthenticationErrMsg            = "Please Authenticate to use the plugin"
	BearerTokenEmptyErrMsg            = "Please enter bearer token"
	AccessKeyEmptyErrMsg              = "Please enter Access Key"
	AccessIDEmptyErrMsg               = "Please enter AccessId"
	HealthAPIErrMsg                   = "Issue with Health API call to Logicmonitor"
	HealthAPIURLErrMsg                = "Issue with Health API URL configuration"
	NoSuchHostError                   = ": no such host"
	WriteTcpError                     = "can't assign requested address"
	ConnectionRefused                 = "connection"
	NetworkError                      = "Netwrok Error"
	ConnectionTimeout                 = "timeout"
	ConnectionUnReachable             = "connect: network is unreachable"
	ConnectionTimeoutError            = "Connection Timeout, please try again"
	ServiceUnavailable                = "Service Temporarily Unavailable"
	InvalidCompanyName                = "Invalid company name configured"
	APIErrMsg                         = "API Failed with status code = "
	URLConfigurationErrMsg            = "URL configuration missing in Backend"
	ErrorReadingResponseBody          = "error reading response Body = "
	ErrorCreatingHttpRequest          = "Error creating http request"
	HttpClientErrorMakingRequest      = "http client error while making request"
	ErrorUnmarshallingErrorData       = "Error Unmarshalling "
	DataNotPresentCacheErrMsg         = "Data not present in FrameCache"
	InvalidFormatOfDataInFrameCache   = "Invalidate data format in FrameCache"
	DataNotPresentEditorCacheErrMsg   = "Data not present in Editor Cache"
	CallApiAndAppendToEditorCache     = "Append new enrie/s to data in editorCache...."
	CallApiAndAppendToFrameCache      = "Append to data in  FrameCache..."
	RateLimitAuditMsg                 = "Rate limit exceeded! API calls so far = %d. current = %d. Total = %d. Allowed = %d"
	RateLimitErrMsg                   = "rate limit exceeded"
	RateLimitValidation               = "API calls so far in last one minute = %d. current = %d. Total = %d"
	APICallSMoreThanRateLimit         = "%d API calls required! causes rate limit error, please reduce the time range"
	AuthSuccessMsg                    = "Authentication Success"
	InternalServerErrorJsonErrMessage = `{ "error":"%s"}`
	MoreThanOneHostDataSources        = "Selected variable host on variable has more than one hostDatasources for ds = "
	HostHasNoMatchingDataSource       = "Selected variable host has no matching datasource = %s OR no instances. Tip : Disable host variable to use host in the query"
	InstancesNotMatchingWithHosts     = "no matching instances found"
	NoDataFromLM                      = "Got no data from LM"
)

// These constants are from PathEndpoints.ts.
const (
	AllInstanceReq          = "AllInstanceReq"
	AllHostReq              = "AllHostReq"
	AutoCompleteInstanceReq = "AutoCompleteInstanceReq"
	AutoCompleteHostReq     = "AutoCompleteHostReq"
	ServiceOrDeviceGroupReq = "ServiceOrDeviceGroupReq"
	AutoCompleteGroupReq    = "AutoCompleteGroupReq"
	DataSourceReq           = "DataSourceReq"
	HostDataSourceReq       = "HostDataSourceReq"
	DataPointReq            = "DataPointReq"

	// RawDataSingleInstaceReq Below constants are not used in Frontend.
	RawDataSingleInstaceReq = "RawDataReq"
	RawDataMultiInstanceReq = "RawDataMultiInstanceReq"
	HealthCheckReq          = "HealthCheckReq"
)

const (
	// AutoCompleteGroupURL AutoCompleteGroupUrl = Groups, gets both device and service.
	AutoCompleteGroupURL = "autocomplete/names?queryToken=display&filterFlag=ImmediateChild&size=10&_=%d&type=hostChain&query=%s&parentsFilters=[]" //nolint:lll

	// GroupExtraFilters Groups, gets either service / devices.
	GroupExtraFilters = `{"AND":[{"OR":[{"name":"groupType","value":"%s","op":":"},{"name":"id","value":1,"op":":"}]},
								{"name":"userPermission","value":"write","op":":"}, {"OR":[{"name":"fullPath","value":"%s","op":"~"},
								{"name":"name","value":"%s","op":"~"}]}]}`
	ServiceOrDeviceGroupURL = "device/groups?fields=id,fullPath,name&sort=fullPath&size=10&_=%d&extraFilters="

	// HostParentFilters  = Devices.
	HostParentFilters    = `[{"filter":"%s","exclude":false,"token":"fullname","matchFilterAsGlob":true}]`
	AutoCompleteHostsURL = `autocomplete/names?queryToken=display&needIdPrefix=true&size=10&_=%d&type=hostChain&query=%s&parentsFilters=` //nolint:lll

	DataSourceURL = `device/devices/%s/devicedatasources?format=json&fields=id,dataSourceDisplayName,dataSourceId,instanceNumber&size=-1&filter=instanceNumber>:1` //nolint:lll

	HostDataSourceURL = `device/devices/%s/devicedatasources?format=json&fields=id&size=-1&filter=dataSourceId:%d,instanceNumber>:1` //nolint:lll

	InstanceParentFilters = `[{"filter":"%s","exclude":false,"token":"fullname","matchFilterAsGlob":true},{"filter":"%s","exclude":false,"token":"display","matchFilterAsGlob":true},{"filter":"%s","exclude":false,"token":"display","matchFilterAsGlob":false}]` //nolint:lll

	AutoCompleteInstanceURL = `autocomplete/names?queryToken=shortname&needIdPrefix=true&size=10&_=%d&type=hostDsChain&query=%s&parentsFilters=` //nolint:lll

	// DataPointURL DataPoints.
	DataPointURL = "setting/datasources/%d?format=json&fields=dataPoints,collectInterval"

	HealthCheckURL = "device/devices?size=1"

	RawDataSingleInstanceURL = "device/devices/%s/devicedatasources/%d/instances/%s/data?start=%d&end=%d"

	RawDataMultiInstanceURL = "device/devices/%s/devicedatasources/%d/data?start=%d&end=%d"

	RawDataMultiInstanceURLWithDpFilter = "device/devices/%s/devicedatasources/%d/data?start=%d&end=%d&datapoints=%s"

	// AllHostURL = Get All Hosts.
	AllHostURL = "device/devices?format=json&fields=id,displayName&size=-1"

	// AllInstanceURL = Get All Instances by hostId and Host Datasource Id.
	AllInstanceURL = "device/devices/%s/devicedatasources/%d/instances?format=json&fields=id,name&size=-1"
)

const (
	DataSourceAndInstanceDelim byte   = '-'
	InstantAndDpDelim          string = " ~ "
	CacheTTLInSeconds          int64  = 60
)

const (
	QueryDataTTLInMinutes                       = 10
	AdditionalCacheTTLInMinutes                 = 2
	HostDsAndHdsMappingCacheTTLInMinutes        = 10
	LastXMunitesCheckForFrameIdCalculationInSec = 90
	NumberOfRecordsWithRateLimit                = 500
)
