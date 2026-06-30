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
    field: 'uuid',
    type: DataTypes.STRING(40),
    defaultValue: Sequelize.UUIDV4,
    primaryKey: true
  },
  REF_ID: {
    field: 'ref_id',
    type: DataTypes.STRING,
    allowNull: true
  },
  NAME: {
    field: 'name',
    type: DataTypes.STRING,
    allowNull: false
  },
  STATUS: {
    field: 'status',
    type: DataTypes.STRING(20),
    allowNull: false
  },
  DESCRIPTION: {
    field: 'description',
    type: DataTypes.STRING(1023),
    allowNull: true,
  },
  VERSION: {
    field: 'version',
    type: DataTypes.STRING(30),
    allowNull: false,
  },
  TYPE: {
    field: 'type',
    type: DataTypes.STRING(20),
    allowNull: false
  },
  AGENT_VISIBILITY: {
    field: 'agent_visibility',
    type: DataTypes.STRING,
    allowNull: false,
    defaultValue: 'VISIBLE'
  },
  TECHNICAL_OWNER: {
    field: 'technical_owner',
    type: DataTypes.STRING,
    allowNull: true
  },
  TECHNICAL_OWNER_EMAIL: {
    field: 'technical_owner_email',
    type: DataTypes.STRING,
    allowNull: true
  },
  BUSINESS_OWNER: {
    field: 'business_owner',
    type: DataTypes.STRING,
    allowNull: true
  },
  BUSINESS_OWNER_EMAIL: {
    field: 'business_owner_email',
    type: DataTypes.STRING,
    allowNull: true
  },
  SANDBOX_URL: {
    field: 'sandbox_url',
    type: DataTypes.STRING,
    allowNull: true
  },
  PRODUCTION_URL: {
    field: 'production_url',
    type: DataTypes.STRING,
    allowNull: true
  },
  METADATA_SEARCH: {
    field: 'metadata_search',
    type: DataTypes.JSONB,
    allowNull: true
  },
  HANDLE: {
    field: 'handle',
    type: DataTypes.STRING,
    allowNull: false
  },
  ORG_UUID: {
    field: 'org_uuid',
    type: DataTypes.STRING(40),
    allowNull: true,
    references: { model: 'dp_organization', key: 'uuid' }
  },
  CREATED_BY: {
    field: 'created_by',
    type: DataTypes.STRING,
    allowNull: false
  },
  CREATED_AT: {
    field: 'created_at',
    type: DataTypes.DATE,
    allowNull: false,
    defaultValue: Sequelize.NOW
  },
  UPDATED_BY: {
    field: 'updated_by',
    type: DataTypes.STRING,
    allowNull: false
  },
  UPDATED_AT: {
    field: 'updated_at',
    type: DataTypes.DATE,
    allowNull: false,
    defaultValue: Sequelize.NOW
  },
}, {
  timestamps: false,
  tableName: 'dp_api_metadata',
  returning: true,
  indexes: [
      {
          name: 'uq_api_metadata_name_version_org',
          unique: true,
          fields: ['NAME', 'VERSION', 'ORG_UUID']
      },
      {
          name: 'uq_api_metadata_org_ref_id',
          unique: true,
          fields: ['ORG_UUID', 'REF_ID']
      },
      {
          name: 'uq_api_metadata_handle_org',
          unique: true,
          fields: ['HANDLE', 'ORG_UUID']
      },
      {
          name: 'idx_api_metadata_status',
          fields: ['STATUS']
      }
  ]
});

const APILabels = sequelize.define('DP_API_LABEL_MAPPING', {

  UUID: {
      field: 'uuid',
      type: DataTypes.STRING(40),
      defaultValue: Sequelize.UUIDV4,
      primaryKey: true
  },
  API_UUID: {
      field: 'api_uuid',
      type: DataTypes.STRING(40),
      allowNull: false,
      references: {
          model: APIMetadata,
          key: 'uuid',
      }
  },
  LABEL_UUID: {
      field: 'label_uuid',
      type: DataTypes.STRING(40),
      allowNull: false,
      references: {
          model: Labels,
          key: 'uuid',
      }
  },
  CREATED_BY: {
      field: 'created_by',
      type: DataTypes.STRING,
      allowNull: false
  },
  CREATED_AT: {
      field: 'created_at',
      type: DataTypes.DATE,
      allowNull: false,
      defaultValue: Sequelize.NOW
  },
}, {
  timestamps: false,
  tableName: 'dp_api_label_mappings',
  returning: true,
  indexes: [
      {
          name: 'uq_api_label_mappings_label_api',
          unique: true,
          fields: ['LABEL_UUID', 'API_UUID']
      },
      {
          name: 'idx_api_label_mappings_api_uuid',
          fields: ['API_UUID']
      }
  ]
});

const APITags = sequelize.define('DP_API_TAG_MAPPING', {

  UUID: {
      field: 'uuid',
      type: DataTypes.STRING(40),
      defaultValue: Sequelize.UUIDV4,
      primaryKey: true
  },
  API_UUID: {
      field: 'api_uuid',
      type: DataTypes.STRING(40),
      allowNull: false,
      references: {
          model: APIMetadata,
          key: 'uuid',
      }
  },
  TAG_UUID: {
      field: 'tag_uuid',
      type: DataTypes.STRING(40),
      allowNull: false,
      references: {
          model: Tags,
          key: 'uuid',
      }
  },
  CREATED_BY: {
      field: 'created_by',
      type: DataTypes.STRING,
      allowNull: false
  },
  CREATED_AT: {
      field: 'created_at',
      type: DataTypes.DATE,
      allowNull: false,
      defaultValue: Sequelize.NOW
  },
}, {
  timestamps: false,
  tableName: 'dp_api_tag_mappings',
  returning: true,
  indexes: [
      {
          name: 'uq_api_tag_mappings_tag_api',
          unique: true,
          fields: ['TAG_UUID', 'API_UUID']
      },
      {
          name: 'idx_api_tag_mappings_api_uuid',
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
