import { getBackendSrv } from "@grafana/runtime";

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
}
