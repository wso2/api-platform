
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
class IdentityProviderDTO {
    constructor(idp) {
        this.name = idp.NAME;
        this.issuer = idp.ISSUER;
        this.authorizationURL = idp.AUTHORIZATION_URL;
        this.tokenURL = idp.TOKEN_URL;
        this.clientId = idp.CLIENT_ID;
        this.callbackURL = idp.CALLBACK_URL;
        this.scope = idp.SCOPE;
        this.logoutURL = idp.LOGOUT_URL;
        this.logoutRedirectURI = idp.LOGOUT_REDIRECT_URL;
        if (idp.USER_INFOR_URL) {
            this.userInfoURL = idp.USER_INFOR_URL;
        }
        if (idp.SIGNUP_URL) {
            this.signUpURL = idp.SIGNUP_URL
        }
        if (idp.JWKS_URL) {
            this.jwksURL = idp.JWKS_URL;
        }
        if (idp.CERTIFICATE) {
            this.certificate = idp.CERTIFICATE;
        }
    }

    setResponseData(data) {
        this.data = data;
    }

    getResponseData() {
        return this.data;
    }
}

module.exports = IdentityProviderDTO;