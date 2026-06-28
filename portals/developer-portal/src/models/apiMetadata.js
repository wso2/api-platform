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
  UUID: {
    type: DataTypes.STRING(40),
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
    type: DataTypes.STRING(20),
    allowNull: false
  },
  DESCRIPTION: {
    type: DataTypes.STRING,
    allowNull: true,
  },
  VERSION: {
    type: DataTypes.STRING(30),
    allowNull: false,
  },
  TYPE: {
    type: DataTypes.STRING(20),
    allowNull: false
  },
  AGENT_VISIBILITY: {
    type: DataTypes.STRING,
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
    type: DataTypes.JSONB,
    allowNull: true
  },
  HANDLE: {
    type: DataTypes.STRING,
    allowNull: false
  },
  CREATED_BY: {
    type: DataTypes.STRING,
    allowNull: false
  },
  CREATED_AT: {
    type: DataTypes.DATE,
    allowNull: false,
    defaultValue: Sequelize.NOW
  },
  UPDATED_BY: {
    type: DataTypes.STRING,
    allowNull: false
  },
  UPDATED_AT: {
    type: DataTypes.DATE,
    allowNull: false,
    defaultValue: Sequelize.NOW
  },
}, {
  timestamps: false,
  tableName: 'DP_API_METADATA',
  returning: true,
  indexes: [
      {
          name: 'UQ_API_METADATA_NAME_VERSION_ORG',
          unique: true,
          fields: ['NAME', 'VERSION', 'ORG_UUID']
      },
      {
          name: 'UQ_API_METADATA_ORG_REF_ID',
          unique: true,
          fields: ['ORG_UUID', 'REF_ID']
      },
      {
          name: 'UQ_API_METADATA_HANDLE_ORG',
          unique: true,
          fields: ['HANDLE', 'ORG_UUID']
      },
      {
          name: 'IDX_API_METADATA_STATUS',
          fields: ['STATUS']
      }
  ]
});

const APILabels = sequelize.define('DP_API_LABEL_MAPPING', {

  UUID: {
      type: DataTypes.STRING(40),
      defaultValue: Sequelize.UUIDV4,
      primaryKey: true
  },
  API_UUID: {
      type: DataTypes.STRING(40),
      allowNull: false,
      references: {
          model: APIMetadata,
          key: 'UUID',
      }
  },
  LABEL_UUID: {
      type: DataTypes.STRING(40),
      allowNull: false,
      references: {
          model: Labels,
          key: 'UUID',
      }
  },
  CREATED_BY: {
      type: DataTypes.STRING,
      allowNull: false
  },
  CREATED_AT: {
      type: DataTypes.DATE,
      allowNull: false,
      defaultValue: Sequelize.NOW
  },
}, {
  timestamps: false,
  tableName: 'DP_API_LABEL_MAPPING',
  returning: true,
  indexes: [
      {
          name: 'UQ_API_LABEL_MAPPING_LABEL_API',
          unique: true,
          fields: ['LABEL_UUID', 'API_UUID']
      },
      {
          name: 'IDX_API_LABEL_MAPPING_API_UUID',
          fields: ['API_UUID']
      }
  ]
});

const APITags = sequelize.define('DP_API_TAG_MAPPING', {

  UUID: {
      type: DataTypes.STRING(40),
      defaultValue: Sequelize.UUIDV4,
      primaryKey: true
  },
  API_UUID: {
      type: DataTypes.STRING(40),
      allowNull: false,
      references: {
          model: APIMetadata,
          key: 'UUID',
      }
  },
  TAG_UUID: {
      type: DataTypes.STRING(40),
      allowNull: false,
      references: {
          model: Tags,
          key: 'UUID',
      }
  },
  CREATED_BY: {
      type: DataTypes.STRING,
      allowNull: false
  },
  CREATED_AT: {
      type: DataTypes.DATE,
      allowNull: false,
      defaultValue: Sequelize.NOW
  },
}, {
  timestamps: false,
  tableName: 'DP_API_TAG_MAPPING',
  returning: true,
  indexes: [
      {
          name: 'UQ_API_TAG_MAPPING_TAG_API',
          unique: true,
          fields: ['TAG_UUID', 'API_UUID']
      },
      {
          name: 'IDX_API_TAG_MAPPING_API_UUID',
          fields: ['API_UUID']
      }
  ]
});

APILabels.belongsTo(APIMetadata, {
  foreignKey: 'API_UUID',
  onDelete: 'CASCADE'
});

APITags.belongsTo(APIMetadata, {
  foreignKey: 'API_UUID',
  onDelete: 'CASCADE'
});

APIContent.belongsTo(APIMetadata, {
  foreignKey: 'API_UUID',
  onDelete: 'CASCADE'
});
APIMetadata.belongsTo(Organization, {
  foreignKey: 'ORG_UUID'
});
APIMetadata.hasMany(APIContent, {
  foreignKey: 'API_UUID',
  onDelete: 'CASCADE'
});

APIMetadata.belongsToMany(Labels, {
  through: APILabels,
  foreignKey: "API_UUID",
  otherKey: "LABEL_UUID"
});
Labels.belongsToMany(APIMetadata, {
  through: APILabels,
  foreignKey: "LABEL_UUID",
  otherKey: "API_UUID"
 });

APIMetadata.belongsToMany(Tags, {
  through: APITags,
  foreignKey: "API_UUID",
  otherKey: "TAG_UUID"
});
Tags.belongsToMany(APIMetadata, {
  through: APITags,
  foreignKey: "TAG_UUID",
  otherKey: "API_UUID"
});

module.exports = {
  APIMetadata,
  APILabels,
  APITags
};
