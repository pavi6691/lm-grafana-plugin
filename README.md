# LogicMonitor plugin for Grafana

Visualize your LogicMonitor metrics with the leading open source software for time series analytics.

[//]: # (![Dashboard]&#40;https://drive.google.com/file/d/1G-y4z_Vb1UCqYr6F8tT7yh_vWNYItrBf/view?usp=sharing&#41;)

## Features

- Familiar query selection as on LogicMonitor.
- Easy selection by either service/devices, followed by device, datasource, instances and dataPoints.
- Multiple instances are supported on single query.
- Multiple dataPoints are supported on single query.
- Caching of APIs with TTL of Polling interval to avoid Rate Limits.
- Caching of queries across multiple users.
# Rate Limit
- Each Query in the Panel will result to a single API call (multiple instance multiple datapoints)
- API results are cached for the collection interval. So if the refresh interval is less than default LM polling interval (1m) , the data will be   brought from cache. 
- This will result in less API requests even when the dashboard interval is lesser than 1m.
- Cache is optimized to handle more frequent requests while user editing queries and making changes.
- A change in the timeframe will trigger a new API call.

## Getting started
- Once data source plugin is installed, click on Add Data Source
- On data source settings page, provide **Company Name** and authenticate by providing **LMv1 Access id** and **Api Key**


## Community Resources, Feedback, and Support
- Have a question? You also can open issue, but for questions, it would be better to use [Grafana Community](https://community.grafana.com/) portal.
- Need additional support? Contact us at [https://www.logicmonitor.com/support/]https://www.logicmonitor.com/support/

---

Licensed under the Mozilla Public License Version 2.0
