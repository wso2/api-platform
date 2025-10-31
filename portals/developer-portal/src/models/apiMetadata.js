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
const { Sequelize, DataTypes } = require('sequelize');
const sequelize = require('../db/sequelize')
const APIContent = require('../models/apiContent')
const APIImages = require('./apiImages')
const { Organization } = require('./organization')
const Labels = require('./labels');

const APIMetadata = sequelize.define('DP_API_METADATA', {
  API_ID: {
    type: DataTypes.UUID,
    defaultValue: Sequelize.UUIDV4,
    primaryKey: true
  },
  REFERENCE_ID: {
    type: DataTypes.UUID,
    allowNull: true,
    unique: true
  },
  API_NAME: {
    type: DataTypes.STRING,
    allowNull: false
  },
  STATUS: {
    type: DataTypes.STRING,
    allowNull: false
  },
  API_DESCRIPTION: {
    type: DataTypes.STRING,
    allowNull: false,
  },
  API_VERSION: {
    type: DataTypes.INTEGER,
    allowNull: false,
  },
  API_TYPE: {
    type: DataTypes.ENUM,
    values: ['REST', 'WS', 'GraphQL', 'SOAP'],
    allowNull: false
  },
  VISIBILITY: {
    type: DataTypes.ENUM,
    values: ['PUBLIC', 'PRIVATE'],
    allowNull: false
  },
  VISIBLE_GROUPS: {
    type: DataTypes.STRING,
    allowNull: true
  },
  TECHNICAL_OWNER: {
    type: DataTypes.STRING,
    allowNull: true
  },
  TECHNICAL_OWNER_EMAIL: {
    type: DataTypes.STRING,
    allowNull: true
  },
  BUSINESS_OWNER: {
    type: DataTypes.STRING,
    allowNull: true
  },
  BUSINESS_OWNER_EMAIL: {
    type: DataTypes.STRING,
    allowNull: true
  },
  SANDBOX_URL: {
    type: DataTypes.STRING,
    allowNull: true
  },
  PRODUCTION_URL: {
    type: DataTypes.STRING,
    allowNull: false
  },
  PROVIDER: {
    type: DataTypes.STRING,
    allowNull: false
  },
  METADATA_SEARCH: {
    type: DataTypes.JSON,
    allowNull: true
  },
  TAGS: {
    type: DataTypes.STRING,
    allowNull: true
  },
  API_HANDLE: {
    type: DataTypes.STRING,
    allowNull: true
  },
}, {
  timestamps: false,
  tableName: 'DP_API_METADATA',
  returning: true
},
{
  indexes: [
      {
          unique: true,
          fields: ['API_NAME', 'API_VERSION', 'ORG_ID']
      }
  ]
});

const APILabels = sequelize.define('DP_API_LABELS', {

  ID: {
      type: DataTypes.UUID,
      defaultValue: Sequelize.UUIDV4,
      primaryKey: true
  },
  ORG_ID: {
      type: DataTypes.UUID,
      defaultValue: Sequelize.UUIDV4
  },
  API_ID: {
      type: DataTypes.UUID,
      references: {
          model: APIMetadata,
          key: 'API_ID',
      }
  },
  LABEL_ID: {
      type: DataTypes.UUID,
      references: {
          model: Labels,
          key: 'LABEL_ID',
      }
  }
}, {
  timestamps: false,
  tableName: 'DP_API_LABELS',
  returning: true,
  unique: false
});

APILabels.belongsTo(Organization, {
  foreignKey: 'ORG_ID'
});

APILabels.belongsTo(APIMetadata, {
  foreignKey: 'API_ID'
});

APIContent.belongsTo(APIMetadata, {
  foreignKey: 'API_ID',
});
APIImages.belongsTo(APIMetadata, {
  foreignKey: 'API_ID',
});
APIMetadata.belongsTo(Organization, {
  foreignKey: 'ORG_ID'
});
APIMetadata.hasMany(APIImages, {
  foreignKey: 'API_ID',
  onDelete: 'CASCADE'
});
APIMetadata.hasMany(APIContent, {
  foreignKey: 'API_ID',
  onDelete: 'CASCADE'
});

APIMetadata.belongsToMany(Labels, { 
  through: APILabels, 
  foreignKey: "API_ID",
  otherKey: "LABEL_ID"
});
Labels.belongsToMany(APIMetadata, { 
  through: APILabels,
  foreignKey: "LABEL_ID",
  otherKey: "API_ID"
 });

module.exports = {
  APIMetadata,
  APILabels
};
