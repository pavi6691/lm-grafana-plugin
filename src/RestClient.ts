import { getBackendSrv } from "@grafana/runtime";
import { lastValueFrom } from "rxjs";

export class RestClient {

    async call(urll: String) {
        return getBackendSrv().datasourceRequest({
            url: '' + urll,
            method: 'GET',
          }).catch(e => {
              if(e.data.errmsg) {
                  e = e.data.errmsg;
              } else if(e.data.message) {
                  if(e.status === 502) {
                      e = e.data.message + ": Host not reachable";
                  } else {
                      e = e.data.message;
                  }
              } else if(e.statusText) {
                  e = e.statusText;
              } else if(e.data.errorMessage) {
                  e = e.data.errorMessage;
              } else {
                  e = "Unknow Error occured : " + e;
              }
              alert(e);
          });
    }

    async httpGet(urll: String, dsid: number, dsURL: string, isBearerEnabled: boolean): Promise<any> {
        const response = await new RestClient().fetch(urll,dsid,dsURL,isBearerEnabled);
        if(response.data && response.data.status && response.data.status !== 200) { // sometime there will be errors with http status 200, handle it from response body
          alert(response.data.errmsg)
          return {
            status: 'error',
            message: response.data.errmsg
          }
        }
        return response;
    }

    async fetch(urll: String, dsid: number, dsURL: string, isBearerEnabled: boolean): Promise<any> {
        if(isBearerEnabled === true) {
            urll = dsURL + urll;
        } else {
            urll = '/api/datasources/' + dsid + '/resources' + urll;
        }
        const observable =  getBackendSrv().fetch({
            url: '' + urll,
            method: 'GET',
          })
        return await lastValueFrom(observable).catch(e => {
            if(e.status === 502 || e.status === 500) { // 502 from proxy 500 from backend) {
                e = e.data.message + " : Host not reachable / invalid company name configured";
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
        });
    }
}
