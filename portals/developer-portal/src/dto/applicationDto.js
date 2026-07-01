
/*
 * Copyright (c) 2024, WSO2 LLC. (http://www.wso2.com) All Rights Reserved.
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
class ApplicationDTO {
    constructor(app) {
        this.id = app.uuid;
        this.displayName = app.display_name;
        this.handle = app.handle;
        this.description = app.description;
        if (app.dp_app_key_mappings) {
            this.appKeyMappings = app.dp_app_key_mappings.map(map => new AppMappingDTO(map));
        }
    }

    setResponseData(data) {
        this.data = data;
    }

    getResponseData() {
        return this.data;
    }
}

class AppMappingDTO {
    constructor(map) {
        this.asClientId = map.as_client_id;
        this.kmId = map.km_uuid;
        this.type = map.type;
    }

    setResponseData(data) {
        this.data = data;
    }

    getResponseData() {
        return this.data;
    }
}

module.exports = {
    ApplicationDTO
};
