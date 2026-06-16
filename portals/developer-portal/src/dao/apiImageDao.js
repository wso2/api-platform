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
const APIImageMetadata = require('../models/apiImage');
const { APIMetadata } = require('../models/apiMetadata');
const { Sequelize, Op } = require('sequelize');

const store = async (apiImages, apiID, t) => {

    let apiImagesList = [];
    try {
        for (var propertyKey in apiImages) {
            apiImagesList.push({
                IMAGE_TAG: propertyKey,
                IMAGE_NAME: apiImages[propertyKey],
                API_ID: apiID
            })
        }
        const apiImagesResponse = await APIImageMetadata.bulkCreate(apiImagesList, { transaction: t });
        return apiImagesResponse;
    } catch (error) {
        if (error instanceof Sequelize.UniqueConstraintError) {
            throw error;
        }
        throw new Sequelize.DatabaseError(error);
    }
}

const update = async (apiImages, orgID, apiID, t) => {

    let imageCreateList = [];
    try {
        for (const propertyKey in apiImages) {
            let apiImageResponse = await getMetadata(propertyKey, apiImages[propertyKey], orgID, apiID, t);
            if (apiImageResponse == null || apiImageResponse == undefined) {
                imageCreateList.push({
                    IMAGE_NAME: apiImages[propertyKey],
                    API_ID: apiID,
                    IMAGE_TAG: propertyKey
                })
            } else {
                const apiImageDataUpdate = await APIImageMetadata.update({
                    IMAGE_NAME: apiImages[propertyKey],
                    IMAGE_TAG: propertyKey
                }, {
                    where: {
                        [Op.or]: [
                            { IMAGE_TAG: apiImageResponse.IMAGE_TAG },
                            { IMAGE_NAME: apiImageResponse.IMAGE_NAME }
                        ],
                        API_ID: apiID
                    },
                    include: [
                        {
                            model: APIMetadata,
                            where: {
                                ORG_ID: orgID
                            }
                        }
                    ],
                    transaction: t
                });
                if (!apiImageDataUpdate) {
                    throw new Sequelize.EmptyResultError("Error updating API Image Metadata");
                }
            }
        }
        if (imageCreateList.length > 0) {
            await APIImageMetadata.bulkCreate(imageCreateList, { transaction: t });
        }
    } catch (error) {
        if (error instanceof Sequelize.UniqueConstraintError) {
            throw error;
        }
        throw new Sequelize.DatabaseError(error);
    }
}

const getMetadata = async (imageTag, imageName, orgID, apiID, t) => {

    try {
        const apiImageData = await APIImageMetadata.findOne({
            where: {
                [Op.or]: [
                    { IMAGE_TAG: imageTag },
                    { IMAGE_NAME: imageName }
                ],
                API_ID: apiID
            },
            include: [
                {
                    model: APIMetadata,
                    where: {
                        ORG_ID: orgID
                    }
                }
            ],
            transaction: t
        });
        return apiImageData;
    } catch (error) {
        if (error instanceof Sequelize.UniqueConstraintError) {
            throw error;
        }
        throw new Sequelize.DatabaseError(error);
    }
}

const get = async (imageTag, apiID, t) => {
    try {
        const apiImageData = await APIImageMetadata.findOne({
            where: {
                IMAGE_TAG: imageTag,
                API_ID: apiID
            },
            transaction: t
        });
        return apiImageData;
    } catch (error) {
        if (error instanceof Sequelize.UniqueConstraintError) {
            throw error;
        }
        throw new Sequelize.DatabaseError(error);
    }
}

const deleteImage = async (imageTag, apiID, t) => {
    try {
        const apiImageData = await APIImageMetadata.destroy({
            where: {
                IMAGE_TAG: imageTag,
                API_ID: apiID
            },
            transaction: t
        });
        return apiImageData;
    } catch (error) {
        if (error instanceof Sequelize.UniqueConstraintError) {
            throw error;
        }
        throw new Sequelize.DatabaseError(error);
    }
}

module.exports = {
    store,
    update,
    getMetadata,
    get,
    delete: deleteImage,
};
