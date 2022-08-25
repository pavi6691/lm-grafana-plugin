import React, { SyntheticEvent,ChangeEvent, PureComponent } from 'react';
import { InlineSwitch, LegacyForms } from '@grafana/ui';
import { DataSourcePluginOptionsEditorProps } from '@grafana/data';
import { MyDataSourceOptions, MySecureJsonData } from './types';
import { SecretFormField } from '@grafana/ui/components/SecretFormField/SecretFormField';


const { FormField } = LegacyForms;

interface Props extends DataSourcePluginOptionsEditorProps<MyDataSourceOptions, MySecureJsonData> {}

interface State {}

export class ConfigEditor extends PureComponent<Props, State> {
  
  onPathChange = (event: ChangeEvent<HTMLInputElement>) => {
    const { onOptionsChange, options } = this.props;
    const jsonData = {
      ...options.jsonData,
      path: event.target.value,
    };
    onOptionsChange({ ...options, jsonData });
  };

  // Secure field (only sent to the backend)
  onAPIKeyChange = (event: ChangeEvent<HTMLInputElement>) => {
    const { onOptionsChange, options } = this.props;
    onOptionsChange({
      ...options,
      secureJsonData: {
        bearer_token: event.target.value,
      },
    });
  };

//AccessId 
  onAccessIdChange = (event: ChangeEvent<HTMLInputElement>) => {
    const { onOptionsChange, options } = this.props;
    const jsonData = {
      ...options.jsonData,
      accessId: event.target.value,
    };
    onOptionsChange({ ...options, jsonData });
  };

//AccessKey
  onAccessKeyChange = (event: ChangeEvent<HTMLInputElement>) => {
    const { onOptionsChange, options } = this.props;
    const jsonData = {
      ...options.jsonData,
      accessKey: event.target.value,
    };
    onOptionsChange({ ...options, jsonData });
  }

  onResetAPIKey = () => {
    const { onOptionsChange, options } = this.props;
    onOptionsChange({
      ...options,
      secureJsonData: {
        ...options.secureJsonData,
        bearer_token: "",
      }
    });
  };

  onLMV1Change = (event: SyntheticEvent<HTMLInputElement>) => {
    const { onOptionsChange, options } = this.props;
    const jsonData = {
      ...options.jsonData,
      isLMV1Enabled: event.currentTarget.checked,
    };
    onOptionsChange({ ...options, jsonData });
  };

  render() {
    const { options } = this.props;
    const { jsonData, secureJsonData } = options;
    return (
      <div className="gf-form-group">
        <div className="gf-form">
          <FormField
            label="Company Name"
            labelWidth={8}
            inputWidth={20}
            onChange={this.onPathChange}
            value={jsonData.path || ''}
            placeholder="Comapny Name"
          />
        </div>
        <div className="gf-form">
        <div style={{ bottom: '32px' }}>
          <InlineSwitch
            width={40}
            disabled={true}
            high={10}
            default={false}
            checked={jsonData.isLMV1Enabled || false}
            label="Enable LMv1"
            showLabel={true}
            onChange={this.onLMV1Change}
          />
        </div>
        </div>
        {jsonData.isLMV1Enabled === false && <div className="gf-form-inline">
          <div className="gf-form">
            <SecretFormField
              isConfigured={(secureJsonData) as boolean}
              value={secureJsonData?.bearer_token || ''}
              label="Bearer Token"
              placeholder="LM breaer token"
              labelWidth={8}
              inputWidth={20}
              onReset={this.onResetAPIKey}
              onChange={this.onAPIKeyChange}
            />
          </div>
        </div> }
        {jsonData.isLMV1Enabled === true && <div className="gf-form-inline"> 
          <div className="gf-form">
            <FormField
              value={jsonData.accessId || ''}
              label="Access Id"
              placeholder="Enter Access Id"
              labelWidth={8}
              inputWidth={20}
              onChange={this.onAccessIdChange}
            />
          </div>
        </div> }
        {jsonData.isLMV1Enabled === true && <div className="gf-form-inline"> 
          <div className="gf-form">
            <FormField
              value={jsonData.accessKey || ''}
              label="Access Key"
              placeholder="Enter Access Key"
              labelWidth={8}
              inputWidth={20}
              onChange={this.onAccessKeyChange}
            />
          </div>
        </div> }
        </div>
    );
  }
}
