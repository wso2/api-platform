const { Sequelize, DataTypes } = require('sequelize');
const sequelize = require('../db/sequelize');
const { ERROR_MESSAGE } = require('../utils/constants');

const SDKJob = sequelize.define('DP_SDK_JOB', {
    JOB_ID: {
        type: DataTypes.STRING,
        primaryKey: true,
        allowNull: false,
    },
    ORG_ID: {
        type: DataTypes.STRING,
        allowNull: false,
    },
    APP_ID: {
        type: DataTypes.STRING,
        allowNull: false,
    },
    JOB_STATUS: {
        type: DataTypes.ENUM('PENDING', 'MERGING', 'SDK_GENERATION', 'APP_CODE_GENERATION', 'COMPLETED', 'FAILED', 'CANCELED'),
        allowNull: false,
        defaultValue: 'PENDING'
    },
    PROGRESS: {
        type: DataTypes.INTEGER,
        allowNull: false,
        defaultValue: 0,
        validate: {
            min: 0,
            max: 100
        }
    },
    CURRENT_STEP: {
        type: DataTypes.STRING,
        allowNull: true
    },
    ERROR_MESSAGE: {
        type: DataTypes.STRING,
        allowNull: true
    }
}, {
    timestamps: true,
    tableName: 'DP_SDK_JOB',
    createdAt: 'CREATED_AT',
    updatedAt: 'UPDATED_AT'
});

module.exports = SDKJob;
