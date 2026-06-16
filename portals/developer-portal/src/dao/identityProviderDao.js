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
const { IdentityProvider } = require('../models/identityProvider');
const { Sequelize } = require('sequelize');

const create = async (orgId, idpData, t) => {
    try {
        const idpResponse = await IdentityProvider.create({
            ORG_ID: orgId,
            ISSUER: idpData.issuer,
            NAME: idpData.name,
            AUTHORIZATION_URL: idpData.authorizationURL,
            TOKEN_URL: idpData.tokenURL,
            ...(idpData.userInfoURL && { USER_INFOR_URL: idpData.userInfoURL }),
            CLIENT_ID: idpData.clientId,
            CALLBACK_URL: idpData.callbackURL,
            ...(idpData.signUpURL && { SIGNUP_URL: idpData.signUpURL }),
            LOGOUT_URL: idpData.logoutURL,
            LOGOUT_REDIRECT_URL: idpData.logoutRedirectURI,
            ...(idpData.scope && { SCOPE: idpData.scope }),
            ...(idpData.jwksURL && { JWKS_URL: idpData.jwksURL }),
            ...(idpData.certificate && { CERTIFICATE: idpData.certificate })
        }, { transaction: t });
        return idpResponse;
    } catch (error) {
        if (error instanceof Sequelize.UniqueConstraintError) {
            throw error;
        }
        throw new Sequelize.DatabaseError(error);
    }
}

const update = async (orgID, idpData, t) => {
    try {
        const [updatedRowsCount, idpContent] = await IdentityProvider.update(
            {
                ORG_ID: idpData.orgId,
                ISSUER: idpData.issuer,
                NAME: idpData.name,
                AUTHORIZATION_URL: idpData.authorizationURL,
                TOKEN_URL: idpData.tokenURL,
                ...(idpData.userInfoURL && { USER_INFOR_URL: idpData.userInfoURL }),
                CLIENT_ID: idpData.clientId,
                CALLBACK_URL: idpData.callbackURL,
                ...(idpData.signUpURL && { SIGNUP_URL: idpData.signUpURL }),
                LOGOUT_URL: idpData.logoutURL,
                LOGOUT_REDIRECT_URI: idpData.logoutRedirectURI,
                SCOPE: idpData.scope,
                ...(idpData.jwksURL && { JWKS_URL: idpData.jwksURL }),
                ...(idpData.certificate && { CERTIFICATE: idpData.certificate })
            },
            {
                where: { ORG_ID: orgID },
                returning: true,
                transaction: t,
            }
        );
        if (updatedRowsCount < 1) {
            throw new Sequelize.EmptyResultError('IdentityProvider not found');
        }
        return [updatedRowsCount, idpContent];
    } catch (error) {
        if (error instanceof Sequelize.UniqueConstraintError) {
            throw error;
        }
        throw new Sequelize.DatabaseError(error);
    }
}

const get = async (orgID) => {
    try {
        const idpResponse = await IdentityProvider.findAll({
            where: {
                ORG_ID: orgID,
            }
        });
        return idpResponse;
    } catch (error) {
        if (error instanceof Sequelize.UniqueConstraintError) {
            throw error;
        }
        throw new Sequelize.DatabaseError(error);
    }
};

const deleteIdp = async (orgID) => {
    try {
        const idpResponse = await IdentityProvider.destroy({
            where: {
                ORG_ID: orgID
            }
        });
        return idpResponse;
    } catch (error) {
        if (error instanceof Sequelize.UniqueConstraintError) {
            throw error;
        }
        throw new Sequelize.DatabaseError(error);
    }
}

module.exports = {
    create,
    update,
    get,
    delete: deleteIdp,
};
