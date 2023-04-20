export class Constants {
    static readonly AllInstanceReq = '/AllInstanceReq';
    static readonly AllHostReq = '/AllHostReq';
    static readonly AutoCompleteInstanceReq = '/AutoCompleteInstanceReq';
    static readonly AutoCompleteHostReq = '/AutoCompleteHostReq'
    static readonly ServiceOrDeviceGroupReq = '/ServiceOrDeviceGroupReq'
    static readonly AutoCompleteGroupReq = '/AutoCompleteGroupReq';
    static readonly DataSourceReq = '/DataSourceReq'
    static readonly DataPointReq = '/DataPointReq'

    static readonly ToolTipForHostVariableSwitch = 'Currently single variable on dashboard is allowed. which is considered to be host. use custom type to add \
    hostname and id as key value pair. By desabling this flag so that data is fetched for host in the query but not host selected on dashboard variable. This \
    helps in cases 1) If selected host from variable is not matching with datasource selected in this query. 2) Instance names not matching with regex/selection \
    made. Note: If dashboard is intended for perticular host then do not disable this flag. And This flag has no effect if there are no variable added on \
    dashboard'


    static readonly EnableBearerToken = false
    static readonly EnableAutocomplete = true

    static readonly EnableRegexFeature = true // Alows user to make instance selections on regex
    static readonly EnableHostVariableFeature = true // Allow host variable. need to configure manually on variable section (Ex- Device1 : 123, Device2 : 321)

    // New Algorithm. if EnableStrategicApiCallFeature is true, Call API for only data that is not available in cache and allow query on cache for specific duration
    static readonly EnableStrategicApiCallFeature = true
    static readonly EnableApiCallThrottler = true // Handle API calls on throttler strategy in santaba
    static readonly MaxNumberOfApiCallPerQuery = -1 // Allowed API calls  per query for historical data. -1 for unlimited
    static readonly ConcurrentApiCallsPerQuery = 1 // Concurrent API calls per query if historical data is enabled. -1 for unlimited
}
