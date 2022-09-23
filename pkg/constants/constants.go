package constants

const (
	RootUrl             = "https://%s.logicmonitor.com/santaba/rest/"
	RawDataFullPath     = "device/devices/%s/devicedatasources/%d/instances/%s/data?start=%d&end=%d"
	RawDataResourcePath = "device/devices/%s/devicedatasources/%d/instances/%s/data"
	DevicesSizeOnePath  = "device/devices?size=1"
	DeviceDevicesPath   = "device/devices"
	AutoCompleteNames   = "autocomplete/names"
)

const (
	AccessKey   = "accessKey"
	BearerToken = "bearer_token"
)

const (
	NoData   = "No Data"
	Response = "response"
	Time     = "time"
)

const (
	Authorization  = "Authorization"
	LMv1           = "LMv1"
	XVersion       = "x-version"
	XVersionValue3 = "3"
	UserAgent      = "User-Agent"
	Get            = "GET"
)

const (
	NoCompanyNameEnteredErrMsg = "Company name not entered"
	NoAuthenticationErrMsg     = "Please Authenticate to use the plugin"
	BearerTokenEmptyErrMsg     = "Please enter bearer token"
	AccessKeyEmptyErrMsg       = "Please enter Access Key"
	AccessIDEmptyErrMsg        = "Please enter AccessId"
	HealthAPIErrMsg            = "Issue with Health API call to Logicmonitor"
	HostUnreachableErrMsg      = "Host not reachable / invalid company name configured"
	UnmarshallingErrMsg        = "Error Unmarshalling healthcheck response"
	APIErrMsg                  = "API Failed with status code = "
	InvalidTokenErrMsg         = "Invalid Token for Company or " //nolint:gosec
	AuthSuccessMsg             = "Authentication Success"
)
