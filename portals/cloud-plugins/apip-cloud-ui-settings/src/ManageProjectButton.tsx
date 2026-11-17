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
 * KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

import { Button } from "@wso2/oxygen-ui";
import { Settings } from "@wso2/oxygen-ui-icons-react";

import { projectManageUrl } from "./cloudConsoleUrl";

const LABEL = "Manage project";

export type ManageProjectButtonProps = {
  /** The project in scope, by handle. */
  projectHandle: string;
  /** The WSO2 Cloud console's origin, when it is not this page's own. */
  consoleUrl?: string;
};

/**
 * The project heading's action: a link to the WSO2 Cloud console's manage page
 * for the project, where it is edited and deleted. A plain anchor, because the
 * Cloud console serves that path, not this console's router.
 */
export default function ManageProjectButton({
  projectHandle,
  consoleUrl,
}: ManageProjectButtonProps) {
  return (
    <Button
      color="primary"
      component="a"
      href={projectManageUrl(projectHandle, consoleUrl)}
      startIcon={<Settings size={16} />}
      variant="outlined"
    >
      {LABEL}
    </Button>
  );
}
