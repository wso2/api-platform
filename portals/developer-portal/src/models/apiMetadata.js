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
  uuid: {
    type: DataTypes.STRING(40),
    defaultValue: Sequelize.UUIDV4,
    primaryKey: true
  },
  ref_id: {
    type: DataTypes.STRING,
    allowNull: true
  },
  name: {
    type: DataTypes.STRING,
    allowNull: false
  },
  status: {
    type: DataTypes.STRING(20),
    allowNull: false
  },
  description: {
    type: DataTypes.STRING(1023),
    allowNull: true,
  },
  version: {
    type: DataTypes.STRING(30),
    allowNull: false,
  },
  type: {
    type: DataTypes.STRING(20),
    allowNull: false
  },
  agent_visibility: {
    type: DataTypes.STRING,
    allowNull: false,
    defaultValue: 'VISIBLE'
  },
  technical_owner: {
    type: DataTypes.STRING,
    allowNull: true
  },
  technical_owner_email: {
    type: DataTypes.STRING,
    allowNull: true
  },
  business_owner: {
    type: DataTypes.STRING,
    allowNull: true
  },
  business_owner_email: {
    type: DataTypes.STRING,
    allowNull: true
  },
  sandbox_url: {
    type: DataTypes.STRING,
    allowNull: true
  },
  production_url: {
    type: DataTypes.STRING,
    allowNull: true
  },
  metadata_search: {
    type: DataTypes.JSONB,
    allowNull: true
  },
  handle: {
    type: DataTypes.STRING,
    allowNull: false
  },
  org_uuid: {
    type: DataTypes.STRING(40),
    allowNull: true,
    references: { model: 'dp_organizations', key: 'uuid' }
  },
  created_by: {
    type: DataTypes.STRING,
    allowNull: false
  },
  created_at: {
    type: DataTypes.DATE,
    allowNull: false,
    defaultValue: Sequelize.NOW
  },
  updated_by: {
    type: DataTypes.STRING,
    allowNull: false
  },
  updated_at: {
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
          fields: ['name', 'version', 'org_uuid']
      },
      {
          name: 'uq_api_metadata_org_ref_id',
          unique: true,
          fields: ['org_uuid', 'ref_id']
      },
      {
          name: 'uq_api_metadata_handle_org',
          unique: true,
          fields: ['handle', 'org_uuid']
      },
      {
          name: 'idx_api_metadata_status',
          fields: ['status']
      }
  ]
});

const APILabels = sequelize.define('DP_API_LABEL_MAPPING', {

  uuid: {
      type: DataTypes.STRING(40),
      defaultValue: Sequelize.UUIDV4,
      primaryKey: true
  },
  api_uuid: {
      type: DataTypes.STRING(40),
      allowNull: false,
      references: {
          model: APIMetadata,
          key: 'uuid',
      }
  },
  label_uuid: {
      type: DataTypes.STRING(40),
      allowNull: false,
      references: {
          model: Labels,
          key: 'uuid',
      }
  },
  created_by: {
      type: DataTypes.STRING,
      allowNull: false
  },
  created_at: {
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
          fields: ['label_uuid', 'api_uuid']
      },
      {
          name: 'idx_api_label_mappings_api_uuid',
          fields: ['api_uuid']
      }
  ]
});

const APITags = sequelize.define('DP_API_TAG_MAPPING', {

  uuid: {
      type: DataTypes.STRING(40),
      defaultValue: Sequelize.UUIDV4,
      primaryKey: true
  },
  api_uuid: {
      type: DataTypes.STRING(40),
      allowNull: false,
      references: {
          model: APIMetadata,
          key: 'uuid',
      }
  },
  tag_uuid: {
      type: DataTypes.STRING(40),
      allowNull: false,
      references: {
          model: Tags,
          key: 'uuid',
      }
  },
  created_by: {
      type: DataTypes.STRING,
      allowNull: false
  },
  created_at: {
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
          fields: ['tag_uuid', 'api_uuid']
      },
      {
          name: 'idx_api_tag_mappings_api_uuid',
          fields: ['api_uuid']
      }
  ]
});

APILabels.belongsTo(APIMetadata, {
  foreignKey: 'api_uuid',
  onDelete: 'CASCADE'
});

APITags.belongsTo(APIMetadata, {
  foreignKey: 'api_uuid',
  onDelete: 'CASCADE'
});

APIContent.belongsTo(APIMetadata, {
  foreignKey: 'api_uuid',
  onDelete: 'CASCADE'
});
APIMetadata.belongsTo(Organization, {
  foreignKey: 'org_uuid'
});
APIMetadata.hasMany(APIContent, {
  foreignKey: 'api_uuid',
  onDelete: 'CASCADE'
});

APIMetadata.belongsToMany(Labels, {
  through: APILabels,
  foreignKey: "api_uuid",
  otherKey: "label_uuid"
});
Labels.belongsToMany(APIMetadata, {
  through: APILabels,
  foreignKey: "label_uuid",
  otherKey: "api_uuid"
 });

APIMetadata.belongsToMany(Tags, {
  through: APITags,
  foreignKey: "api_uuid",
  otherKey: "tag_uuid"
});
Tags.belongsToMany(APIMetadata, {
  through: APITags,
  foreignKey: "tag_uuid",
  otherKey: "api_uuid"
});

module.exports = {
  APIMetadata,
  APILabels,
  APITags
};
