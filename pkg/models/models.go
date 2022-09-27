package models

type LabelStringValue struct {
	Label string `json:"label"`
	Value string `json:"value"`
}

type LabelIntValue struct {
	Label string `json:"label"`
	Value int64  `json:"value"`
}

type DataSource struct {
	Ds    int64  `json:"ds"`
	Label string `json:"label"`
	Value int64  `json:"value"`
}

type Data struct {
	DataSourceName string          `json:"dataSourceName,omitempty"`
	DataPoints     []string        `json:"dataPoints,omitempty"`
	Values         [][]interface{} `json:"values,omitempty"`
	Time           []int64         `json:"time,omitempty"`
}

type SingleInstaceRawData struct {
	Data Data `json:"data,omitempty"`
}

type ValuesAndTime struct {
	Values [][]interface{} `json:"values,omitempty"`
	Time   []int64         `json:"time,omitempty"`
}

type MultiInstanceData struct {
	DataSourceName string                   `json:"dataSourceName,omitempty"`
	DataPoints     []string                 `json:"dataPoints,omitempty"`
	Instances      map[string]ValuesAndTime `json:"instances,omitempty"`
}

type MultiInstanceRawData struct {
	Data MultiInstanceData `json:"data,omitempty"`
}

type QueryModel struct {
	TypeSelected             string             `json:"typeSelected"`
	GroupSelected            LabelIntValue      `json:"groupSelected"`
	HostSelected             LabelStringValue   `json:"hostSelected"`
	HdsSelected              int64              `json:"hdsSelected"`
	DataSourceSelected       DataSource         `json:"dataSourceSelected"`
	InstanceSelected         []LabelStringValue `json:"instanceSelected"`
	InstanceSearch           string             `json:"instanceSearch"`
	DataPointSelected        []LabelIntValue    `json:"dataPointSelected"`
	WithStreaming            bool               `json:"withStreaming"`
	CollectInterval          int64              `json:"collectInterval"`
	LastQueryEditedTimeStamp int64              `json:"lastQueryEditedTimeStamp"`
}

type DeviceData struct {
	DeviceData string `json:"data"`
	Errmsg     string `json:"errmsg"`
	Status     int32  `json:"status"`
}

type PluginSettings struct {
	Path            string `json:"path"`
	AccessID        string `json:"accessId"`
	IsBearerEnabled bool   `json:"isBearerEnabled"`
	IsLMV1Enabled   bool   `json:"isLMV1Enabled"` //nolint:tagliatelle
	Version         string `json:"version"`
}

type AuthSettings struct {
	AccessKey   string
	BearerToken string
}
