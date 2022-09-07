import { DataQuery, DataSourceJsonData } from '@grafana/data';
export interface MyQuery extends DataQuery {
  typeSelected: any;
  serviceGroup: boolean | false;
  deviceGroup: boolean | false;
  groupSelected: any;
  hostSelected: any;
  hdsSelected: any;
  dataSourceSelected: any;
  instanceSelected: any;
  instanceSearch: any;
  dataPointSelected: any[];
  collectInterval: number;
  uniqueId: string;
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
