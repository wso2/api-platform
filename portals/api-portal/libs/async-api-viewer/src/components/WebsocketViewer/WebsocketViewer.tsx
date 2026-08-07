/*
 * Copyright (c) 2025, WSO2 LLC. (http://www.wso2.com). All Rights Reserved.
 *
 * This software is the property of WSO2 LLC. and its suppliers, if any.
 * Dissemination of any information or reproduction of any material contained
 * herein is strictly forbidden, unless permitted by WSO2 in accordance with
 * the WSO2 Commercial License available at http://wso2.com/licenses.
 * For specific language governing the permissions and limitations under
 * this license, please see the license as well as any agreement you’ve
 * entered into with WSO2 governing the purchase of this software and any
 * associated services.
 */

import React, { useState, useMemo, useEffect } from 'react';
import { Box, CircularProgress } from '@material-ui/core';
import cloneDeep from 'lodash.clonedeep';
import TopicViewer from './TopicViewer';

export interface WebSocketViewerProps {
  token?: string;
  apiEndpoint?: string;
  sandboxEndpoint?: string;
  asyncapi?: AsyncApi;
  isDevportal?: boolean;
  asyncApiType: APITypeEnum | undefined;
}

export enum APITypeEnum {
  WS = 'WS',
  WEBSUB = 'WEBSUB',
}

interface SwaggerSecuritySchemasValue {
  type: string;
  name: string;
  in: string;
  scheme?: string;
  bearerFormat?: string;
}
interface AsyncApiChannel {
  subscribe?: object;
  publish?: object;
  parameters?: object;
}
export interface AsyncApi {
  asyncapi: string;
  info: {
    title: string;
    version: string;
    description?: string;
    license?: {
      name: string;
      url?: string;
    };
  };
  servers?: {
    [key: string]: {
      url: string;
      description?: string;
      protocol: string;
    };
  };
  channels: {
    [key: string]: AsyncApiChannel;
  };
}

interface Channel {
  name: string;
  subscribe: boolean;
  publish: boolean;
  parameters?: object;
}

export interface SwaggerSecuritySchemas {
  key: string;
  value: SwaggerSecuritySchemasValue;
}

function mapAsyncApiTopics(asyncapi: AsyncApi) {
  const { channels } = asyncapi;
  const allTopicsList: Channel[] = Object.entries(channels).map(
    ([channelName, channel]: [string, any]) => ({
      name: channelName,
      subscribe: !!channel?.subscribe,
      publish: !!channel?.publish,
      parameters: channel.parameters,
    })
  );

  return allTopicsList;
}

function WebSocketViewer(props: WebSocketViewerProps) {
  const { 
    token = "", 
    apiEndpoint = "", 
    sandboxEndpoint = "",
    asyncapi = {
      asyncapi: "2.0.0",
      info: {
        title: "WebSocket API",
        version: "1.0.0"
      },
      channels: {
        '/': {
          subscribe: {
            message: 'A sample message',
            operationId: 'test'
          },
          publish: {
            message: 'A sample message',
            operationId: 'test'
          }
        }
      }
    },
    isDevportal = false,
    asyncApiType = APITypeEnum.WS
  } = props;

  const [allTopics, setAllTopics] = useState([
    { name: '/*', subscribe: true, publish: true, parameters: {} },
  ] as Channel[]);

  useEffect(() => {
    if (asyncapi?.channels) {
      setAllTopics(mapAsyncApiTopics(asyncapi));
    }
  }, [asyncapi]);

  const newAsyncapiObj = useMemo(() => {
    if (asyncapi && Object.keys(asyncapi).length > 0 && apiEndpoint) {
      let newAsyncapi = cloneDeep(asyncapi);
      if (newAsyncapi.asyncapi?.startsWith('2')) {
        newAsyncapi = {
          ...newAsyncapi,
          servers: {
            default: { url: apiEndpoint, protocol: 'ws' },
            sandbox: { url: sandboxEndpoint, protocol: 'ws' },
          },
        };
      } else if (newAsyncapi.asyncapi?.startsWith('3')) {
        newAsyncapi = {
          ...newAsyncapi,
          servers: {
            default: { url: apiEndpoint, protocol: 'ws' },
            sandbox: { url: sandboxEndpoint, protocol: 'ws' },
          },
        };
      }
      if (newAsyncapi?.channels) {
        setAllTopics(mapAsyncApiTopics(newAsyncapi));
      }
      return newAsyncapi;
    } else {
      let newAsyncapi = cloneDeep(asyncapi);
      setAllTopics(mapAsyncApiTopics(newAsyncapi));
    }
    return null;
  }, [asyncapi, apiEndpoint, sandboxEndpoint]);

  useEffect(() => {
    if (newAsyncapiObj?.channels) {
      setAllTopics(mapAsyncApiTopics(newAsyncapiObj));
    }
  }, [newAsyncapiObj]);

  function buildPayload(topic?: string) {
    if (asyncApiType === APITypeEnum.WEBSUB) {
      return JSON.stringify({
        'hub.mode': 'subscribe',
        'hub.topic': topic || 'sample-topic',
        'hub.callback': 'http://example.com/callback',
        'hub.secret': 'xxxxxxxxx',
        'hub.lease_seconds': 864000,
      });
    }
    return '{ "message": "Hello Server" }';
  }

  if (!allTopics || !asyncapi?.channels) {
    return (
      <Box
        display="flex"
        alignItems="center"
        justifyContent="center"
        padding={8}
      >
        <CircularProgress size={25} />
      </Box>
    );
  }

  if (apiEndpoint) {
    return (
      <Box>
        {allTopics.map(({ name, publish, subscribe, parameters }: Channel) => (
          <TopicViewer
            key={name}
            token={token}
            apiEndpoint={apiEndpoint}
            sandboxEndpoint={sandboxEndpoint || ''}
            topic={name}
            publish={publish}
            subscribe={subscribe}
            parameters={parameters}
            isDevportal={isDevportal}
            payload={buildPayload(name)}
            asyncType={asyncApiType || APITypeEnum.WS}
          />
        ))}
      </Box>
    );
  }

  return <></>;
};

export default WebSocketViewer;
