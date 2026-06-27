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
const sequelize = require('../db/sequelizeConfig')
const APIContent = require('../models/apiContent')
const { Organization } = require('./organization')
const Labels = require('./label');
const Tags = require('./tag');

const APIMetadata = sequelize.define('DP_API_METADATA', {
  ID: {
    type: DataTypes.UUID,
    defaultValue: Sequelize.UUIDV4,
    primaryKey: true
  },
  REF_ID: {
    type: DataTypes.STRING,
    allowNull: true
  },
  NAME: {
    type: DataTypes.STRING,
    allowNull: false
  },
  STATUS: {
    type: DataTypes.ENUM,
    values: ['CREATED', 'PUBLISHED'],
    allowNull: false
  },
  DESCRIPTION: {
    type: DataTypes.STRING,
    allowNull: true,
  },
  VERSION: {
    type: DataTypes.STRING,
    allowNull: false,
  },
  TYPE: {
    type: DataTypes.ENUM,
    values: ['REST', 'WS', 'GRAPHQL', 'SOAP', 'WEBSUB', 'MCP'],
    allowNull: false
  },
  AGENT_VISIBILITY: {
    type: DataTypes.ENUM,
    values: ['VISIBLE', 'HIDDEN'],
    allowNull: false,
    defaultValue: 'VISIBLE'
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
    allowNull: true
  },
  METADATA_SEARCH: {
    type: DataTypes.JSON,
    allowNull: true
  },
  HANDLE: {
    type: DataTypes.STRING,
    allowNull: true
  },
}, {
  timestamps: false,
  tableName: 'DP_API_METADATA',
  returning: true,
  indexes: [
      {
          name: 'UQ_API_METADATA_NAME_VERSION_ORG',
          unique: true,
          fields: ['NAME', 'VERSION', 'ORG_ID']
      },
      {
          name: 'UQ_API_METADATA_ORG_REF_ID',
          unique: true,
          fields: ['ORG_ID', 'REF_ID']
      },
      {
          name: 'UQ_API_METADATA_HANDLE_ORG',
          unique: true,
          fields: ['HANDLE', 'ORG_ID']
      }
  ]
});

const APILabels = sequelize.define('DP_API_LABEL_MAPPING', {

  ID: {
      type: DataTypes.UUID,
      defaultValue: Sequelize.UUIDV4,
      primaryKey: true
  },
  ORG_ID: {
      type: DataTypes.UUID,
      allowNull: false
  },
  API_ID: {
      type: DataTypes.UUID,
      allowNull: false,
      references: {
          model: APIMetadata,
          key: 'ID',
      }
  },
  LABEL_ID: {
      type: DataTypes.UUID,
      allowNull: false,
      references: {
          model: Labels,
          key: 'ID',
      }
  }
}, {
  timestamps: false,
  tableName: 'DP_API_LABEL_MAPPING',
  returning: true,
  indexes: [
      {
          name: 'UQ_API_LABEL_MAPPING_LABEL_API_ORG',
          unique: true,
          fields: ['LABEL_ID', 'API_ID', 'ORG_ID']
      }
  ]
});

const APITags = sequelize.define('DP_API_TAG_MAPPING', {

  ID: {
      type: DataTypes.UUID,
      defaultValue: Sequelize.UUIDV4,
      primaryKey: true
  },
  ORG_ID: {
      type: DataTypes.UUID,
      allowNull: false
  },
  API_ID: {
      type: DataTypes.UUID,
      allowNull: false,
      references: {
          model: APIMetadata,
          key: 'ID',
      }
  },
  TAG_ID: {
      type: DataTypes.UUID,
      allowNull: false,
      references: {
          model: Tags,
          key: 'ID',
      }
  }
}, {
  timestamps: false,
  tableName: 'DP_API_TAG_MAPPING',
  returning: true,
  indexes: [
      {
          name: 'UQ_API_TAG_MAPPING_TAG_API_ORG',
          unique: true,
          fields: ['TAG_ID', 'API_ID', 'ORG_ID']
      }
  ]
});

APILabels.belongsTo(Organization, {
  foreignKey: 'ORG_ID'
});

APILabels.belongsTo(APIMetadata, {
  foreignKey: 'API_ID',
  onDelete: 'CASCADE'
});

APITags.belongsTo(Organization, {
  foreignKey: 'ORG_ID'
});

APITags.belongsTo(APIMetadata, {
  foreignKey: 'API_ID',
  onDelete: 'CASCADE'
});

APIContent.belongsTo(APIMetadata, {
  foreignKey: 'API_ID',
  onDelete: 'CASCADE'
});
APIMetadata.belongsTo(Organization, {
  foreignKey: 'ORG_ID'
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

APIMetadata.belongsToMany(Tags, {
  through: APITags,
  foreignKey: "API_ID",
  otherKey: "TAG_ID"
});
Tags.belongsToMany(APIMetadata, {
  through: APITags,
  foreignKey: "TAG_ID",
  otherKey: "API_ID"
});

module.exports = {
  APIMetadata,
  APILabels,
  APITags
};
