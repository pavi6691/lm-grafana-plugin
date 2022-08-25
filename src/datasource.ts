import {
  DataQueryRequest,
  DataQueryResponse,
  DataSourceApi,
  DataSourceInstanceSettings,
  FieldType,
  MutableDataFrame,
  // LoadingState,
  // CircularDataFrame,
} from '@grafana/data';
import { MyQuery, MyDataSourceOptions } from './types';
// import { Observable, merge } from 'rxjs';
// import { getBackendSrv } from '@grafana/runtime';
import { RestClient } from 'RestClient';

export class DataSource extends DataSourceApi<MyQuery, MyDataSourceOptions> {
  url?: string;
  id: number;
  storedJsonData: any;
  constructor(instanceSettings: DataSourceInstanceSettings<MyDataSourceOptions>) {
    super(instanceSettings);
    this.url = instanceSettings.url;
    this.id = instanceSettings.id;
    this.storedJsonData = instanceSettings.jsonData;
  }

  // Query Rest service using stream model

  // query(options: DataQueryRequest<MyQuery>): Observable<DataQueryResponse> {
  //   const observables = options.targets.map((query) => {
  //       return new Observable<DataQueryResponse>((subscriber) => {
  //         this.doRequest(query,options).then((response) => {
  //           if(response && response.data && response.data.data != null) {
  //             const frame = new CircularDataFrame({
  //               append: 'tail',
  //               capacity: 1000,
  //             });
  //             frame.refId = query.refId;
  //             frame.addField({ name: 'time', type: FieldType.time });
  //             for (var d in response.data.data.dataPoints) {
  //               for(var dp in query.dataPointSelected) {
  //                 if(query.dataPointSelected[dp].label === response.data.data.dataPoints[d]) {
  //                   frame.addField({ name: response.data.data.dataPoints[d], type: FieldType.number });
  //                 }
  //               }
  //             }
  //             var metricValues: number[][];
  //             var intervalId: string | number | NodeJS.Timer | undefined;
  //             for (var i in response.data.data.time) {
  //               var row = [response.data.data.time[i]];
  //               metricValues = response.data.data.values[i];
  //               for (var j in metricValues) {
  //                 for (var k in frame.fields) {
  //                   if(frame.fields[k].name === response.data.data.dataPoints[j]) {
  //                     row.push(metricValues[j]);
  //                   }
  //                 }
  //               }
  //               intervalId = setInterval(() => {
  //                 frame.appendRow(row);
  //                 subscriber.next({
  //                   data: [frame],
  //                   key: query.refId,
  //                   state: LoadingState.Streaming,
  //                 });
  //               },5000);
  //             }
  //             return () => {
  //               clearInterval(intervalId);
  //             };
  //           } else if(response && response.data && response.data.errmsg && response.data.errmsg.length > 0) {
  //             throw new Error(response.data.errmsg);
  //           } else {
  //             return new Observable<DataQueryResponse>();
  //           }
  //         });
  //       });
  //   });
  //   return merge(...observables);
  // }

  async query(options: DataQueryRequest<MyQuery>): Promise<DataQueryResponse> {
    var metricValues: number[][];
    const promises = options.targets.map((query) =>
      this.doRequest(query,options).then((response) => {
        const fields = [{ name: 'time', type: FieldType.time }];
        if(response && response.data && response.data.data != null) {
          for (var d in response.data.data.dataPoints) {
            for(var dp in query.dataPointSelected) {
              if(query.dataPointSelected[dp].label === response.data.data.dataPoints[d]) {
                  fields.push({ name: response.data.data.dataPoints[d], type: FieldType.number });
              }
            }
          }
          const frame = new MutableDataFrame({
            refId: query.refId,
            fields: fields,
          });
          for (var i in response.data.data.time) {
            var row = [response.data.data.time[i]];
            metricValues = response.data.data.values[i];
            for (var j in metricValues) {
              for (var k in fields) {
                if(fields[k].name === response.data.data.dataPoints[j]) {
                  row.push(metricValues[j]);
                }
              }
            }
            frame.appendRow(row);
          }
          return frame;
        } else if(response && response.data && response.data.errmsg && response.data.errmsg.length > 0) {
          throw new Error(response.data.errmsg);
        } else {
          return new MutableDataFrame({
            refId: query.refId,
            fields: fields,
          });
        }
      })
    );
    return Promise.all(promises).then((data) => ({ data }));
  }

  async doRequest(query: MyQuery, options: DataQueryRequest<MyQuery>) {
    if(query.dataPointSelected) {
      if(query.groupSelected === undefined || query.groupSelected == null) {
        throw new Error("Please select group")
      }
  
      if(query.hostSelected === undefined || query.hostSelected == null) {
        throw new Error("Please select host")
      }
      if(query.hdsSelected === undefined || query.hdsSelected == null) {
        throw new Error("Please select datasource")
      }
      if(query.instanceSelected === undefined || query.instanceSelected === null || query.instanceSelected.length === 0) {
        throw new Error("Please select instance")
      }
      if(query.dataPointSelected !== undefined && query.dataPointSelected !== null && query.dataPointSelected.length > 0) {
        const routePath =  '/device/devices/' + query.hostSelected.value + 
           '/devicedatasources/' + query.hdsSelected + 
           '/instances/' +  query.instanceSelected.value + 
           '/data' +
           '?start=' + options.range.from.unix() + '&end=' + options.range.to.unix()
        return new RestClient().fetch(routePath,this.id || 0,this.url || '', this.storedJsonData.isLMV1Enabled);
      } else {
        throw new Error("Please select datapoints");
      }
    }
    return undefined;
  }

  async testDatasource() {
  //  const companyName = this.storedJsonData.path;
   const companyRoute =  '/device/devices/'+'?size=1';

   var statusVal = "";
   var messageVal = "";

   return new RestClient().fetch(companyRoute,this.id || 0,this.url || '', this.storedJsonData.isLMV1Enabled);

    // await getBackendSrv().datasourceRequest({
    //   url : companyRoute,
    //   method : 'GET' 
    // }).then((response)=>{
    //   statusVal = 'Authentication Success!'
    //   messageVal = 'Authentication Success!'
    // }).catch((error)=>{
    //   statusVal = 'error'
    //   if(error.status === 400){
    //     messageVal = 'Invalid Token for Comapny'+' '+companyName 
    //   }
    //   else if(error.status === 502){
    //     messageVal = 'Invalid Comapny'+' '+companyName
    //   }else{
    //     messageVal = 'Unknow Error occured'
    //   }
      
    // })

    return {
      status: statusVal,
      message: messageVal
    }
  }
}
