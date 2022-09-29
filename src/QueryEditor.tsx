import React, { SyntheticEvent, PureComponent, useState, useEffect } from 'react';
import { Select, InlineLabel, MultiSelect, RadioButtonGroup } from '@grafana/ui';
import { QueryEditorProps, SelectableValue } from '@grafana/data';
import { DataSource } from './datasource';
import { MyDataSourceOptions, MyQuery } from './types';
import { RestClient } from 'RestClient';
import {PathEndpoints} from './PathEndpoints';
type Props = QueryEditorProps<DataSource, MyQuery, MyDataSourceOptions>;
const SELECT_ALL_STAR = "*";

export class QueryEditor extends PureComponent<Props> {
  getRawData = () => {
    const { onChange, query, onRunQuery } = this.props;
    query.lastQueryEditedTimeStamp = new Date().getTime();
    onChange({ ...query });
    onRunQuery();
  };

  onWithStreamingChange = (event: SyntheticEvent<HTMLInputElement>) => {
    const { onChange, query, onRunQuery } = this.props;
    onChange({ ...query, withStreaming: event.currentTarget.checked });
    onRunQuery();
  };

  async doAutoCompleteRequest(urll: String, idAsPrifix: boolean) {
    const result = await new RestClient().httpGet(urll,  this.props.datasource.id, this.props.datasource.url || '', this.props.datasource.storedJsonData.isBearerEnabled, this.props.query);
    const hostArray = [];
    if(result.data) {
      for (var i = 0; i < result.data.items.length; i++) {
        if(idAsPrifix) {
          var lm_host = '' + result.data.items[i];
          if (lm_host !== undefined) {
            var newarr = lm_host.split(":");
            var label = "";
            if(newarr.length >  1) {
              for (var j = 1; j < newarr.length; j++) {
                if(j === newarr.length - 1) {
                  label = label + newarr[j];
                } else {
                  label = label + newarr[j] + ":";
                }
              }
              hostArray.push({ value: newarr[0], label: label });
            }
          }
        } else {
          hostArray.push({label: result.data.items[i] });
        }
      }
    }
    return hostArray;
  }
  callPromise = (urll: String, idAsPrifix: boolean) => {
    return new Promise<Array<SelectableValue<string>>>((resolve) => {
      setTimeout(() => {
        resolve(this.doAutoCompleteRequest(urll,idAsPrifix));
      }, 1500);
    });
  }

  async doGroupRequest(urll: String) {
    const result = await new RestClient().httpGet(urll,  this.props.datasource.id, this.props.datasource.url || '', this.props.datasource.storedJsonData.isBearerEnabled, this.props.query);
    const hostArray = [];
    if(result.data) {
      for (var i = 0; i < result.data.data.total; i++) {
        const group = result.data.data.items[i];
        if (group !== undefined && group.fullPath !== "") {
          hostArray.push({ value: group.id, label: group.fullPath });
        }
      }
    }
    return hostArray;
  }

  async doDeviceRequest(urll: String) {
    const result = await new RestClient().httpGet(urll,  this.props.datasource.id, this.props.datasource.url || '', this.props.datasource.storedJsonData.isBearerEnabled, this.props.query);
    const hostArray = [];
    if(result.data) {
      for (var i = 0; i < result.data.data.total; i++) {
        const lm_host = result.data.data.items[i];
        if (lm_host !== undefined) {
          hostArray.push({ value: lm_host.id.toString(), label: lm_host.displayName });
        }
      }
    }
    return hostArray;
  }

  async doDataSourceRequest(urll: String) {
    const result = await new RestClient().httpGet(urll,  this.props.datasource.id, this.props.datasource.url || '', this.props.datasource.storedJsonData.isBearerEnabled, this.props.query);
    const hostArray = [];
    if(result.data) {
      for (var i = 0; i < result.data.data.total; i++) {
        const lm_host = result.data.data.items[i];
        if (lm_host !== undefined) {
          hostArray.push({ value: lm_host.id, label: lm_host.dataSourceDisplayName, ds: lm_host.dataSourceId });
        }
      }
    }
    return hostArray;
  }

  async doInstanceRequest(urll: String) {
    const result = await new RestClient().httpGet(urll,  this.props.datasource.id, this.props.datasource.url || '', this.props.datasource.storedJsonData.isBearerEnabled, this.props.query);
    const hostArray = [];
    if(result.data) {
      for (var i = 0; i < result.data.data.total; i++) {
        const lm_host = result.data.data.items[i];
        if (lm_host !== undefined) {
          hostArray.push({ value: lm_host.id.toString(), label: lm_host.name });
        }
      }
    } 
    return hostArray;
  }

  async doDataPointRequest(urll: String) {
    const result = await new RestClient().httpGet(urll,  this.props.datasource.id, this.props.datasource.url || '', this.props.datasource.storedJsonData.isBearerEnabled, this.props.query);
    const hostArray = [];
    if(result.data) {
      for (var i = 0; i < result.data.data.dataPoints.length; i++) {
        const lm_host = result.data.data.dataPoints[i];
        if (lm_host !== undefined) {
          hostArray.push({value: lm_host.id, label: lm_host.name});
        }
      }
      this.props.query.collectInterval = result.data.data.collectInterval;
    }
    return hostArray;
  }

  hostSelectAsync = () => {
    
    const [groupSelected, setGroupSelected] = useState<any>();
    const [hostSelected, setHostSelected] = useState<any>();
    const [dataSourceSelected, setDataSourceSelected] = useState<any>();
    const [instanceSelected, setInstanceSelected] = useState<any[]>();
    const [dataPointSelected, setDataPointSelected] = useState<any[]>();

    const [groupOptions, setGroupOptions] = useState<Array<SelectableValue<any>>>();
    const [hostOptions, setHostOptions] = useState<Array<SelectableValue<any>>>();
    const [dsOptions, setDsOptions] = useState<Array<SelectableValue<any>>>();
    const [instanceOptions, setInstanceOptions] = useState<Array<SelectableValue<any>>>();
    const [dpOptions, setDpOptions] = useState<Array<SelectableValue<any>>>();

    const [isGroupLoading,setGroupLoading] = useState(false);
    const [isDeviceLoading,setDeviceLoading] = useState(false);
    const [isDSLoading,setDsLoading] = useState(false);
    const [isInstanceLoading,setInstanceLoading] = useState(false);
    const [isDPLoading,setDPLoading] = useState(false);

    const [isAutocompleteEnabled] = useState(true); // currently only used for group and hosts. as it requires devices datasource id, for datasources using standerd api

    const optionStartsWithValue = (option: SelectableValue<string>, value: string) =>
            option.label?.toString().startsWith(value) || false;
    
    const loadGroups = () => {
      if(this.props.query.deviceGroup === true && this.props.query.serviceGroup === true) {
        setGroupLoading(true);
        this.callPromise(PathEndpoints.AutoCompleteGroupReq, false).then((rs) => {
          setGroupOptions(rs);
        }).finally(() => {  
          setGroupLoading(false);
        });
      } else if(this.props.query.deviceGroup === true || this.props.query.serviceGroup === true) {
        setGroupLoading(true);
         const loadGroupAsyncOptions = () => {
         return new Promise<Array<SelectableValue<string>>>((resolve) => {
          setTimeout(() => {
            resolve(this.doGroupRequest(PathEndpoints.ServiceOrDeviceGroupReq));
            }, 1500);
          });
         };
        loadGroupAsyncOptions().then((rs) => {
          setGroupOptions(rs);
        }).finally(() => {
          setGroupLoading(false);
        });
      }
    }

    const setGroup = (value: any) => {
      setGroupSelected(value);
      setHostSelected(null);
      setDataSourceSelected(null);
      setInstanceSelected([]);
      setDataPointSelected([]);

      setHostOptions(undefined);
      setDsOptions(undefined);
      setInstanceOptions(undefined);
      setDpOptions(undefined);

      this.props.query.groupSelected = value;
      this.props.query.hostSelected = null as any;
      this.props.query.dataSourceSelected = null as any;
      this.props.query.hdsSelected = null as any;
      this.props.query.instanceSelected = null as any;
      this.props.query.instanceSearch = null as any;
      this.props.query.dataPointSelected = null as any;
    }

    const loadHosts = () => {
      setDeviceLoading(true)
      this.callPromise(PathEndpoints.AutoCompleteHostReq, true).then((rs) => {
        setHostOptions(rs);
      }).finally(() => {
        setDeviceLoading(false);
      });
    }

    // Enable when device data source id is not required to fetch raw data
    // const loadDatasources = () => { 
    //   setDsLoading(true)
    //   const autocomplete = '/autocomplete/names?queryToken=fullname&filterFlag=DataSourceWithInstance&needIdPrefix=true&size=10&_=' + new Date().getTime() + '&type='
    //   const parentsFilter = '[{"filter":"' + this.props.query.groupSelected.label + '","exclude":false,"token":"fullname","matchFilterAsGlob":true},' 
    //   + '{"filter":"'+ this.props.query.hostSelected.label + '","exclude":false,"token":"display","matchFilterAsGlob":true}'
    //   + ']';
    //   this.callPromise(autocomplete + 'hostDsChain&query=' + this.props.query.dataSourceSelected + '&parentsFilters=' + encodeURI(parentsFilter)).then((rs) => {
    //     setDsOptions(rs);
    //   }).finally(() => {
    //     setDsLoading(false);
    //   });
    // }
    
    const loadInstances = () => {
      setInstanceLoading(true)
      this.callPromise(PathEndpoints.AutoCompleteInstanceReq, true).then((rs) => {
        setInstanceOptions(rs);
      }).finally(() => {
        setInstanceLoading(false);
      });
    }

    if(isAutocompleteEnabled === false) {
      useEffect(() => {
        const loadHostAsyncOptions = () => {
          setDeviceLoading(true)
          const rootPath = PathEndpoints.AllHostReq;
          return new Promise<Array<SelectableValue<string>>>((resolve) => {
            setTimeout(() => {
              resolve(this.doDeviceRequest(rootPath));
            }, 1500);
          });
        };
        loadHostAsyncOptions().then((rs) => {
          setHostOptions(rs);
          setHostSelected(this.props.query.hostSelected);
        }).finally(() => {
          setDeviceLoading(false);
        });
      }, []);
      useEffect(() => {
        if(dataSourceSelected) {
          const loadInstancesAsyncOptions = () => {
            setInstanceLoading(true)
            const routePath = PathEndpoints.AllInstanceReq;
            return new Promise<Array<SelectableValue<string>>>((resolve) => {
              setTimeout(() => {
                resolve(this.doInstanceRequest(routePath));
              }, 1500);
            });
          };
          loadInstancesAsyncOptions().then((rs) => {
            setInstanceOptions(rs);
            setDataSourceSelected(this.props.query.dataSourceSelected)
          }).finally(() => {
            setInstanceLoading(false);
          });
        }
      }, [dataSourceSelected]);
    } else {
      useEffect(() => {
        if(this.props.query.groupSelected) {
           setGroupSelected(SELECT_ALL_STAR);
        } else {
           this.props.query.groupSelected = {label:SELECT_ALL_STAR}
        }
        loadGroups();
        // eslint-disable-next-line react-hooks/exhaustive-deps
      }, []);
      useEffect(() => {
        if(groupSelected) {
          if(this.props.query.hostSelected) {
            setHostSelected(SELECT_ALL_STAR)
          } else {
            this.props.query.hostSelected = {label:SELECT_ALL_STAR}
          }
          loadHosts();
        }
      }, [groupSelected]);
      useEffect(() => {
        if(dataSourceSelected) {
          if(this.props.query.instanceSearch) {
            setDataSourceSelected(SELECT_ALL_STAR);
          } else {
            this.props.query.instanceSearch = SELECT_ALL_STAR
          }
          loadInstances();
        }
      }, [dataSourceSelected]);
    }

    useEffect(() => {
      if(hostSelected) {
        const loadDataSourceAsyncOptions = () => {
          setDsLoading(true)
          return new Promise<Array<SelectableValue<string>>>((resolve) => {
            setTimeout(() => {
              resolve(this.doDataSourceRequest(PathEndpoints.DataSourceReq));
            }, 1500);
          });
        };
        loadDataSourceAsyncOptions().then((rs) => {
          setDsOptions(rs);
          setDataSourceSelected(this.props.query.dataSourceSelected);
        }).finally(() => {
          setDsLoading(false);
        });
      }
    }, [hostSelected]);

    useEffect(() => {
      if(dataSourceSelected) {
        const loadDpIdsAsyncOptions = () => {
          setDPLoading(true)
          return new Promise<Array<SelectableValue<string>>>((resolve) => {
            setTimeout(() => {
              resolve(this.doDataPointRequest(PathEndpoints.DataPointReq));
            }, 1500);
          });
        };
        loadDpIdsAsyncOptions().then((rs) => {
          setDpOptions(rs);
        }).finally(() => {
          setDPLoading(false);
        });
      }
    }, [dataSourceSelected]);
    if(this.props.query.deviceGroup === undefined) {
      this.props.query.deviceGroup = true
      this.props.query.typeSelected = "Normal";
    }
    return (
      <div style={{ width: '100%' }}>
        {isAutocompleteEnabled && <div style={{ display: 'flex', marginBottom:5 }}>
          <InlineLabel width={15}>Resource Type</InlineLabel>
          <RadioButtonGroup
            onChange={(v) => {
              this.props.query.typeSelected = v;
              setGroup(null)
              loadGroups();
            }}
            value={this.props.query.typeSelected}
            options={[
              { label: 'Devices', value: 'Normal' },
              { label: 'Services', value: 'BizService' }, ]}
            fullWidth={true}
          />
        </div>}
        {isAutocompleteEnabled && <div style={{ display: 'flex', marginBottom:5 }}>
          <InlineLabel width={15}>Groups</InlineLabel>
          <Select
            menuPlacement={'bottom'}
            defaultValue={this.props.query.groupSelected}
            options={groupOptions}
            // filterOption={optionStartsWithValue}
            placeholder="Groups"
            isLoading={isGroupLoading}
            noOptionsMessage='No groups found'
            loadingMessage='Fetching groups...'
            value={groupSelected}
            allowCustomValue={true}
            onInputChange={(v) => {
              if(v.length >  0) {
                this.props.query.groupSelected.label = v;
                loadGroups();
              }
            }}
            onChange={(v) => {
              if(v !== null && this.props.query.groupSelected !== v) {
                setGroup(v)
              }
            }}
          />
        </div>}
        <div style={{ display: 'flex', marginBottom:5 }}>
          <InlineLabel width={15}>Resources</InlineLabel>
          <Select
            menuPlacement={'bottom'}
            defaultValue={this.props.query.hostSelected}
            options={hostOptions}
            // filterOption={optionStartsWithValue}
            placeholder="Resources"
            isLoading={isDeviceLoading}
            noOptionsMessage='No resources found'
            loadingMessage='Fetching resources...'
            allowCustomValue={true}
            value={hostSelected}
            onInputChange={(v) => {
              if(isAutocompleteEnabled && v.length >  0) {
                this.props.query.hostSelected.label = v;
                loadHosts();
              }
            }}
            onChange={(v) => {
              if(v !== null && this.props.query.hostSelected !== v ) {
                setHostSelected(v);
                setDataSourceSelected(null);
                setInstanceSelected([]);
                setDataPointSelected([]);

                setDsOptions(undefined);
                setInstanceOptions(undefined);
                setDpOptions(undefined);

                this.props.query.hostSelected = v;
                this.props.query.dataSourceSelected = null as any;
                this.props.query.hdsSelected = null as any;
                this.props.query.instanceSelected = null as any;
                this.props.query.instanceSearch = null as any;
                this.props.query.dataPointSelected = null as any;
              }
            }}
          />
          </div>
          <div style={{ display: 'flex', marginBottom:5 }}>
          <InlineLabel width={15}>DataSources</InlineLabel>
          <Select
            menuPlacement={'bottom'}
            defaultValue={this.props.query.dataSourceSelected}
            options={dsOptions}
            filterOption={optionStartsWithValue}
            placeholder="DataSources"
            isLoading={isDSLoading}
            loadingMessage='Fetching data sources...'
            noOptionsMessage='No datasources found'
            value={dataSourceSelected}
            onChange={(v) => {
              if(v !== null) {
                setDataSourceSelected(v);
                setInstanceSelected([]);
                setDataPointSelected([]);

                setInstanceOptions(undefined);
                setDpOptions(undefined);

                this.props.query.dataSourceSelected = v;
                this.props.query.hdsSelected = v.value
                this.props.query.instanceSelected = null as any;
                this.props.query.instanceSearch = null as any;
                this.props.query.dataPointSelected = null as any;
              }
            }}
          />
          </div>
          <div style={{ display: 'flex', marginBottom:5 }}>
          <InlineLabel width={15}>Instances</InlineLabel>
          <MultiSelect
            menuPlacement={'bottom'}
            defaultValue={this.props.query.instanceSelected}
            options={instanceOptions}
            // filterOption={optionStartsWithValue}
            placeholder="Instances"
            isLoading={isInstanceLoading}
            loadingMessage='Fetching instances...'
            noOptionsMessage='No instances found'
            value={instanceSelected}
            allowCustomValue={true}
            onInputChange={(v) => {
              if(isAutocompleteEnabled && v.length >  0) {
                this.props.query.instanceSearch = v;
                loadInstances();
              }
            }}
            onChange={(v) => {
              setInstanceSelected(v);
              this.props.query.instanceSelected = v;
              if(this.props.query.dataPointSelected) {
                this.getRawData();
              }
            }}
          />
          </div>
          <div style={{ display: 'flex', marginBottom:5 }}>
          <InlineLabel width={15}>DataPoints</InlineLabel>
          <MultiSelect
            menuPlacement={'bottom'}
            defaultValue={this.props.query.dataPointSelected}
            options={dpOptions}
            filterOption={optionStartsWithValue}
            placeholder="DataPoints"
            isLoading={isDPLoading}
            loadingMessage='Fetching data points...'
            noOptionsMessage='No datapoints found'
            isClearable={true}
            value={dataPointSelected}
            onChange={(v) => {
              setDataPointSelected(v);
              this.props.query.dataPointSelected = v;
              this.getRawData();
            }}
          />
        </div>
      </div>
    );
  };
  render() {
    // const query = defaults(this.props.query, defaultQuery);
    // const { withStreaming } = query;
    return (
      <div className="gf-form">
        <this.hostSelectAsync />
        {/* <div style={{ bottom: '32px' }}>
          <InlineSwitch
            width={40}
            disabled={true}
            high={10}
            default={false}
            checked={withStreaming || false}
            label="Enable Streaming"
            showLabel={true}
            onChange={this.onWithStreamingChange}
          />
        </div> */}
      </div>
    );
  }
}
