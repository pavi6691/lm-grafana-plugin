import { DataQuery, DataSourceJsonData } from '@grafana/data';
export interface MyQuery extends DataQuery {
  groupSelected: any;
  hostSelected: any;
  hdsSelected: any;
  dataSourceSelected: any;
  instanceSelected: any;
  instanceSearch: any;
  dataPointSelected: any[];
  withStreaming: boolean;
}
export const defaultQuery: Partial<MyQuery> = {
  withStreaming: false,
};
/**
 * These are options configured for each DataSource instance.
 */
export interface MyDataSourceOptions extends DataSourceJsonData {
  path?: string;
  lmDataSourceName?: string;
}
/**
 * Value that is used in the backend, but never sent over HTTP to the frontend
 */
export interface MySecureJsonData {
  apiKey?: string;
}
