/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except in compliance
 * with the License. You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package cloudbase

import (
	"fmt"
)

// PipelineNotListedError says a project is bound to a deployment pipeline the platform does not
// list.
type PipelineNotListedError struct{ Pipeline string }

// Error reports that the pipeline the project is bound to is not one the platform lists.
func (e *PipelineNotListedError) Error() string {
	return fmt.Sprintf("deployment pipeline %q is not listed", e.Pipeline)
}

// PipelineBindingMissingError says the platform has no deployment pipeline binding for a project.
type PipelineBindingMissingError struct{ Cause error }

// Error reports why the project has no pipeline binding.
func (e *PipelineBindingMissingError) Error() string { return e.Cause.Error() }

// Unwrap returns the cause of the missing binding.
func (e *PipelineBindingMissingError) Unwrap() error { return e.Cause }
