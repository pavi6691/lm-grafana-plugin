import { DataQuery, DataSourceJsonData } from '@grafana/data';
/*todo check with Praveen to rename this*/
export interface MyQuery extends DataQuery {
  typeSelected: any;
  serviceGroup: boolean | false;
  deviceGroup: boolean | false;
  groupSelected: any;
  hostSelected: any;
  hdsSelected: any;
  dataSourceSelected: any;
  instanceSelected: any[];
  instanceSelectBy: any;
  instanceRegex: any;
  validInstanceRegex: boolean;
  instanceSearch: any;
  dataPointSelected: any[];
  collectInterval: number;
  lastQueryEditedTimeStamp: any;
  isQueryInterpolated: boolean
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
  accessId?: string;
  isLMV1Enabled?: boolean;
  isBearerEnabled?: boolean;
}
/**
 * Value that is used in the backend, but never sent over HTTP to the frontend
 */
export interface MySecureJsonData {
  bearer_token?: string;
  accessKey?: string;
}
