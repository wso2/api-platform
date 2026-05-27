const { DataTypes } = require('sequelize');
const sequelize = require('../db/sequelize');

const BillingEngineKey = sequelize.define('BillingEngineKey', {
  ID: {
    type: DataTypes.UUID,
    defaultValue: DataTypes.UUIDV4,
    primaryKey: true
  },
  ORG_ID: {
    type: DataTypes.UUID,
    allowNull: false
  },
  BILLING_ENGINE: {
    type: DataTypes.STRING(32),
    allowNull: false
  },
  SECRET_KEY_ENC: {
    type: DataTypes.TEXT,
    allowNull: false
  },
  PUBLISHABLE_KEY_ENC: {
    type: DataTypes.TEXT,
    allowNull: false
  },
  WEBHOOK_SECRET_ENC: {
    type: DataTypes.TEXT,
    allowNull: false
  },
  CREATED_AT: {
    type: DataTypes.DATE,
    defaultValue: DataTypes.NOW
  },
  UPDATED_AT: {
    type: DataTypes.DATE,
    defaultValue: DataTypes.NOW
  }
}, {
  tableName: 'DP_BILLING_ENGINE_KEYS',
  indexes: [
    { fields: ['ORG_ID', 'BILLING_ENGINE'], unique: true }
  ],
  timestamps: false
});

module.exports = BillingEngineKey;
