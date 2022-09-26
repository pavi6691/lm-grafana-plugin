package constants

//
//type LmUrl struct {
//	name string
//	url  string
//}
//
//func (v LmUrl) Name() string {
//	return v.name
//}
//
//func (v LmUrl) URL() string {
//	return v.url
//}
//
//var (
//	AutoCompleteGroupURL = LmUrl{
//		"AutoCompleteGroupURL",
//		"autocomplete/names?queryToken=display&filterFlag=ImmediateChild&size=10&_=%d&type=hostChain&query=%s&parentsFilters=[]"}
//
//	// GroupExtraFilters = Groups, gets either service / devices
//	GroupExtraFilters = LmUrl{
//		"GroupExtraFilters",
//		`{"AND":[{"OR":[{"name":"groupType","value":"%s","op":":"},{"name":"id","value":1,"op":":"}]},
//								{"name":"userPermission","value":"write","op":":"}, {"OR":[{"name":"fullPath","value":"%s","op":"~"},
//								{"name":"name","value":"%s","op":"~"}]}]}`,
//	}
//
//	ServiceOrDeviceGroupURL = LmUrl{
//		"GroupExtraFilters",
//		"device/groups?fields=id,fullPath,name&sort=fullPath&size=10&_=%d&extraFilters=",
//	}
//
//	// HostParentFilters  = Devices
//	HostParentFilters = LmUrl{
//		"HostParentFilters",
//		`[{"filter":"%s","exclude":false,"token":"fullname","matchFilterAsGlob":true}]`,
//	}
//
//	AutoCompleteHostsURL = LmUrl{
//		"AutoCompleteHostsURL",
//		`autocomplete/names?queryToken=display&needIdPrefix=true&size=10&_=%d&type=hostChain&query=%s&parentsFilters=`,
//	}
//
//	DataSourceURL = LmUrl{
//		"DataSourceURL",
//		`device/devices/%s/devicedatasources?format=json&fields=id,dataSourceDisplayName,dataSourceId,instanceNumber&size=-1&filter=instanceNumber>:1`,
//	}
//
//	InstanceParentFilters = LmUrl{
//		"InstanceParentFilters",
//		`[{"filter":"%s","exclude":false,"token":"fullname","matchFilterAsGlob":true},
//				{"filter":"%s","exclude":false,"token":"display","matchFilterAsGlob":true},
//				{"filter":"%s","exclude":false,"token":"display","matchFilterAsGlob":false}]`,
//	}
//
//	AutoCompleteInstanceURL = LmUrl{
//		"AutoCompleteInstanceURL",
//		`autocomplete/names?queryToken=shortname&needIdPrefix=true&size=10&_=%d&type=hostDsChain&query=%s&parentsFilters=`,
//	}
//
//	// DataPointURL DataPoints
//	DataPointURL = LmUrl{
//		"DataPointURL",
//		"setting/datasources/%d?format=json&fields=dataPoints,collectInterval",
//	}
//
//	// AllHostURL = Get All Hosts // -------- If autocomplete is disabled below APIs are used
//	AllHostURL = LmUrl{
//		"AllHostURL",
//		"device/devices?format=json&fields=id,displayName&size=-1",
//	}
//
//	// AllInstanceURL = Get All Instances by hostId and Host Datasource Id
//	AllInstanceURL = LmUrl{
//		"AllInstanceURL",
//		"device/devices/%s/devicedatasources/%d/instances?format=json&fields=id,name&size=-1",
//	}
//
//	HealthCheckUrl1 = LmUrl{
//		"HealthCheckUrl1",
//		"device/devices?size=1",
//	}
//)
