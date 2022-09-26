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
          alert(response.data.errmsg)
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
        if( e.status === 500) {
            e = e.data.message
        } else if(e.status === 502) { // 502 from proxy 500 from backend) {
            e =  "Host not reachable / invalid company name configured";
        } else if(e.status === 400 ) {
            e = 'Invalid Token for Comapny or ' + e.data.message; 
        } else if(e.data.error) {
            e = e.data.error;
        } else if(e.data.errmsg) {
            e = e.data.errmsg;
        } else if(e.data.message) {
            e = e.data.message
        } else if(e.data.errorMessage) {
            e = e.data.errorMessage;
        } else if(e.statusText) {
            e = e.statusText;
        } else {
            e = 'HTTP Error : ' + e.status;
        }
        alert(e);
        return {
            status: "error",
            message: e
          }
    }
}
