/*
 * Copyright (c) 2026, WSO2 LLC. (http://www.wso2.com). All Rights Reserved.
 *
 * This software is the property of WSO2 LLC. and its suppliers, if any.
 * Dissemination of any information or reproduction of any material contained
 * herein is strictly forbidden, unless permitted by WSO2 in accordance with
 * the WSO2 Commercial License available at http://wso2.com/licenses.
 * For specific language governing the permissions and limitations under
 * this license, please see the license as well as any agreement you've
 * entered into with WSO2 governing the purchase of this software and any
 * associated services.
 */

import React from 'react';

interface RichFormBoundaryProps {
  fallback: React.ReactNode;
  children: React.ReactNode;
}

interface RichFormBoundaryState {
  failed: boolean;
}

/**
 * Shows the fallback (the standard editor) if the rich form fails to load or
 * render, for example because a policy's formSchema is invalid.
 */
export default class RichFormBoundary extends React.Component<RichFormBoundaryProps, RichFormBoundaryState> {
  state: RichFormBoundaryState = { failed: false };

  static getDerivedStateFromError(): RichFormBoundaryState {
    return { failed: true };
  }

  componentDidCatch(error: unknown) {
    console.warn('The policy form could not be rendered; showing the standard editor instead.', error);
  }

  render() {
    return this.state.failed ? this.props.fallback : this.props.children;
  }
}
