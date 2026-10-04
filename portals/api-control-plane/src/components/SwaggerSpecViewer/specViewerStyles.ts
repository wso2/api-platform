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

/**
 * Shared Swagger UI styles for both lazy-loaded spec editors.
 *
 * Keep both editors importing this module rather than the CSS files directly.
 * The side-effect-only module ensures Rollup includes the styles in each
 * editor's production chunk. Add shared styles to `SwaggerSpecViewer.css`;
 * page-specific overrides belong in a separate stylesheet imported afterward.
 */

import 'swagger-ui-react/swagger-ui.css';
import './SwaggerSpecViewer.css';
