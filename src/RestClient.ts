import { getBackendSrv } from "@grafana/runtime";
import { lastValueFrom } from "rxjs";
import { MyQuery } from "types";

export class RestClient {

    async call(urll: String) {
        return getBackendSrv().datasourceRequest({
            url: '' + urll,
            method: 'GET',
          }).catch(e => {
            return this.getErrorMessage(e);
          });
    }

    async httpGet(urll: String, dsid: number, dsURL: string, isBearerEnabled: boolean, query: MyQuery): Promise<any> {
        const response = await this.fetch(urll,dsid,dsURL,isBearerEnabled,query);
        if(response.data && response.data.status && response.data.status !== 200) { // sometime there will be errors with http status 200, handle it from response body
          return {
            status: 'error',
            message: response.data.errmsg
          }
        }
        return response;
    }

    async fetch(urll: String, dsid: number, dsURL: string, isBearerEnabled: boolean,query: MyQuery): Promise<any> {
        urll = '/api/datasources/' + dsid + '/resources' + urll;
        const observable =  getBackendSrv().fetch({
            url: '' + urll,
            method: 'POST',
            data: JSON.stringify(query)
          })
        return await lastValueFrom(observable).catch(e => {
            return this.getErrorMessage(e);
        });
    }

    getErrorMessage(e: any): any {
        return {
            status: "error",
            message: e
          }
    }
}
