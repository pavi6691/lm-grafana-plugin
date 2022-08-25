import { getBackendSrv } from "@grafana/runtime";
import { lastValueFrom } from "rxjs";

export class RestClient {

    call(urll: String) {
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

    async fetch(urll: String, dsid: number, dsURL: string, isLMv1Enabled: boolean): Promise<any> {
        if(isLMv1Enabled === true) {
            urll = '/api/datasources/' + dsid + '/resources' + urll;
        } else {
            dsURL = dsURL + urll;
        }
        const observable =  getBackendSrv().fetch({
            url: '' + urll,
            method: 'GET',
          });
          const response = await lastValueFrom(observable);
          return response;
    }
}
