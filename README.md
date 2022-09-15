# LogicMonitor plugin for Grafana

Visualize your LogicMonitor metrics with the leading open source software for time series analytics.

![Dashboard](https://drive.google.com/file/d/1G-y4z_Vb1UCqYr6F8tT7yh_vWNYItrBf/view?usp=sharing)

## Features

- Create interactive and support to reuse dashboards by exporting them in the form of json
- Familier query selection as on Logicmonitor
- No hazzles writting query using sql,regex or any other sytax
- Easy selection by either service/devices, followed by device, datasource, instances and datapoints.
- Multiple metrics are supported on single query
- Can have metrics from multiple data sources in single panel

## Installation

Install by using `grafana-cli`

```sh
grafana-cli plugins install logicmonitor-datasource
```

## Getting started
- Once plugin is installed, add it
- On settings page, privice company name and athenticate by providing LMv1 access id and api key
Then you can create your first dashboard with step-by-step

## Community Resources, Feedback, and Support
- Have a question? You also can open issue, but for questions, it would be better to use [Grafana Community](https://community.grafana.com/) portal.
- Need additional support? Contact us at [https://www.logicmonitor.com/support/]https://www.logicmonitor.com/support/

---

Licensed under the Apache 2.0 License