import React, { SyntheticEvent,ChangeEvent, PureComponent } from 'react';
import { InlineLabel, InlineSwitch, LegacyForms } from '@grafana/ui';
import { DataSourcePluginOptionsEditorProps } from '@grafana/data';
import { MyDataSourceOptions, MySecureJsonData } from './types';

const { SecretFormField, FormField } = LegacyForms;

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
    const secureJsonData = {
      ...options.secureJsonData,
      accessKey: event.target.value,
    };
    onOptionsChange({ ...options, secureJsonData });
  }

  onResetAPIKey = () => {
    const { onOptionsChange, options } = this.props;
    onOptionsChange({
      ...options,
      secureJsonFields: {
        ...options.secureJsonFields,
        bearer_token: false,
      },
      secureJsonData: {
        ...options.secureJsonData,
        bearer_token: '',
      },
    });
  };

  onResetAccessKey = () => {
    const { onOptionsChange, options } = this.props;
    onOptionsChange({
      ...options,
      secureJsonFields: {
        ...options.secureJsonFields,
        accessKey: false,
      },
      secureJsonData: {
        ...options.secureJsonData,
        accessKey: '',
      },
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

  onBearerChange = (event: SyntheticEvent<HTMLInputElement>) => {
    const { onOptionsChange, options } = this.props;
    const jsonData = {
      ...options.jsonData,
      isBearerEnabled: event.currentTarget.checked,
    };
    onOptionsChange({ ...options, jsonData });
  };

  render() {
    const { options } = this.props;
    const { jsonData, secureJsonFields } = options;
    const secureJsonData = (options.secureJsonData || {}) as MySecureJsonData;
    return (
      <div className="gf-form-group">
        <div className="gf-form" style={{ display: 'flex', marginBottom:50 }}>
          <FormField
            label="Company Name"
            labelWidth={10}
            inputWidth={20}
            onChange={this.onPathChange}
            value={jsonData.path || ''}
            placeholder="Comapny Name"
          />
        </div>
        <div className="box">
          <div style={{ display: 'flex', marginBottom:2 }}>
              <h4>Authentication</h4>
          </div>
        </div>
        <div className="gf-form">
        <div style={{ display: 'flex', marginBottom:2 }}>
          <InlineLabel width={20}>Bearer token</InlineLabel>
          <InlineSwitch
            defaultChecked={jsonData.isBearerEnabled}
            checked={jsonData.isBearerEnabled}
            showLabel={true}
            onChange={this.onBearerChange}
          />
        </div>
        </div>
        <div className="gf-form">
        <div style={{ display: 'flex', marginBottom:40 }}>
          <InlineLabel width={20}>LMv1 token</InlineLabel>
          <InlineSwitch
            defaultChecked={jsonData.isLMV1Enabled}
            checked={jsonData.isLMV1Enabled}
            showLabel={true}
            onChange={this.onLMV1Change}
          />
        </div>
        </div>
        {jsonData.isBearerEnabled === true && <div className="box">
          <div style={{ display: 'flex', marginBottom:2 }}>
              <h4>Bearer Token</h4>
          </div>
        </div> }
        {jsonData.isBearerEnabled === true && <div className="gf-form-inline" style={{ display: 'flex', marginBottom:40 }}>
          <div className="gf-form">
            <SecretFormField
              isConfigured={(secureJsonFields && secureJsonFields.bearer_token) as boolean}
              value={secureJsonData?.bearer_token || ''}
              label="Bearer Token"
              placeholder="LM breaer token"
              labelWidth={10}
              inputWidth={20}
              onReset={this.onResetAPIKey}
              onChange={this.onAPIKeyChange}
            />
          </div>
        </div> }
        {jsonData.isLMV1Enabled === true && <div className="box">
          <div style={{ display: 'flex', marginBottom:2 }}>
              <h4>LMv1 Token</h4>
          </div>
        </div> }
        {jsonData.isLMV1Enabled === true && <div className="gf-form-inline"> 
          <div className="gf-form">
            <FormField
              value={jsonData.accessId || ''}
              label="Access Id"
              placeholder="Enter Access Id"
              labelWidth={10}
              inputWidth={20}
              onChange={this.onAccessIdChange}
            />
          </div>
        </div> }
        {jsonData.isLMV1Enabled === true && <div className="gf-form-inline"> 
          <div className="gf-form">
            <SecretFormField
              isConfigured={(secureJsonFields && secureJsonFields.accessKey) as boolean} 
              value={secureJsonData.accessKey || ''}
              label="Access Key"
              placeholder="Enter Access Key"
              labelWidth={10}
              inputWidth={20}
              onReset={this.onResetAccessKey}    
              onChange={this.onAccessKeyChange}  
            />
          </div>
        </div> }
        </div>
    );
  }
}
