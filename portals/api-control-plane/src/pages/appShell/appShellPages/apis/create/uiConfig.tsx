/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

import { Box } from '@wso2/oxygen-ui';
import {} from '@wso2/oxygen-ui-icons-react';
import { defineMessages } from 'react-intl';

import gRPCIconUrl from '@/assets/icons/gRPC.svg';
import graphqlIconUrl from '@/assets/icons/graphql.svg';
import httpIconUrl from '@/assets/icons/httpLogo.svg';
import websocketIconUrl from '@/assets/icons/websocket.svg';
import websubIconUrl from '@/assets/icons/websub.svg';
import { ApiType } from './types';

/** Square box the API-type marks are drawn into, on the selector cards. */
const API_TYPE_ICON_SIZE = 42;

/**
 * Brand mark for an API type, from `src/assets/icons`.
 * `aria-hidden` because the card already carries the type's name.
 */
const ApiTypeIcon = ({ src }: { src: string }) => (
  <Box
    alt=""
    aria-hidden
    component="img"
    src={src}
    sx={{
      display: 'block',
      height: API_TYPE_ICON_SIZE,
      objectFit: 'contain',
      width: API_TYPE_ICON_SIZE,
    }}
  />
);

export const HttpIcon = () => <ApiTypeIcon src={httpIconUrl} />;

export const GraphqlIcon = () => <ApiTypeIcon src={graphqlIconUrl} />;

export const WebSocketIcon = () => <ApiTypeIcon src={websocketIconUrl} />;

export const WebSubIcon = () => <ApiTypeIcon src={websubIconUrl} />;

export const GrpcIcon = () => <ApiTypeIcon src={gRPCIconUrl} />;

const messages = defineMessages({
  graphQlDescription: {
    id: 'api.create.apiType.graphQl.description',
    defaultMessage: 'Serve a GraphQL schema through the gateway.',
  },
  graphQlTitle: {
    id: 'api.create.apiType.graphQl.title',
    defaultMessage: 'GraphQL API',
  },
  grpcDescription: {
    id: 'api.create.apiType.grpc.description',
    defaultMessage: 'Expose high-performance gRPC services.',
  },
  grpcTitle: {
    id: 'api.create.apiType.grpc.title',
    defaultMessage: 'gRPC API',
  },
  restDescription: {
    id: 'api.create.apiType.rest.description',
    defaultMessage: 'Expose REST and other HTTP backends.',
  },
  restTitle: {
    id: 'api.create.apiType.rest.title',
    defaultMessage: 'REST API',
  },
  webSocketDescription: {
    id: 'api.create.apiType.webSocket.description',
    defaultMessage: 'Stream data over long-lived WebSocket connections.',
  },
  webSocketTitle: {
    id: 'api.create.apiType.webSocket.title',
    defaultMessage: 'WebSocket API',
  },
  webSubDescription: {
    id: 'api.create.apiType.webSub.description',
    defaultMessage: 'Deliver events to subscribers over WebSub.',
  },
  webSubTitle: {
    id: 'api.create.apiType.webSub.title',
    defaultMessage: 'WebSub API',
  },
});

export const API_TYPES: ApiType[] = [
  {
    key: 'rest',
    title: messages.restTitle,
    description: messages.restDescription,
    rawTitle: 'REST API',
    rawDescription: 'Expose REST and other HTTP backends.',
    icon: <HttpIcon />,
    enabled: true,
  },
  {
    key: 'websocket',
    title: messages.webSocketTitle,
    description: messages.webSocketDescription,
    rawTitle: 'WebSocket API',
    rawDescription: 'Stream data over long-lived WebSocket connections.',
    icon: <WebSocketIcon />,
    enabled: false,
  },
  {
    key: 'graphql',
    title: messages.graphQlTitle,
    description: messages.graphQlDescription,
    rawTitle: 'GraphQL API',
    rawDescription: 'Serve a GraphQL schema through the gateway.',
    icon: <GraphqlIcon />,
    enabled: false,
  },
  {
    key: 'websub',
    title: messages.webSubTitle,
    description: messages.webSubDescription,
    rawTitle: 'WebSub API',
    rawDescription: 'Deliver events to subscribers over WebSub.',
    icon: <WebSubIcon />,
    enabled: false,
  },
  {
    key: 'grpc',
    title: messages.grpcTitle,
    description: messages.grpcDescription,
    rawTitle: 'gRPC API',
    rawDescription: 'Expose gRPC services through the gateway.',
    icon: <GrpcIcon />,
    enabled: false,
  },
];
