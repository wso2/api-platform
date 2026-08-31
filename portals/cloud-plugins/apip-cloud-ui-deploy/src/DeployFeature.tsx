/*
 * Copyright (c) 2026, WSO2 LLC (http://www.wso2.com). All Rights Reserved.
 *
 * This software is the property of WSO2 LLC and its suppliers, if any.
 * Dissemination of any information or reproduction of any material contained
 * herein in any form is strictly forbidden, unless permitted by WSO2 expressly.
 * You may not alter or remove any copyright or other notice from copies of this content.
 */

import type { FC } from 'react';
import DeployPage from './DeployPage';
import type { AIWorkspaceHostPort } from './hostPort';

export type DeployFeatureProps = {
  port: AIWorkspaceHostPort;
};

/** The extension's `render(port)` result: a self-contained, client-only deploy pipeline view. */
const DeployFeature: FC<DeployFeatureProps> = ({ port }) => {
  return <DeployPage notify={(message) => port.notify(message, 'success')} />;
};

export default DeployFeature;
