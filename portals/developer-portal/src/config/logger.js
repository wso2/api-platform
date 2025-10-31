/*
 * Copyright (c) 2025, WSO2 LLC. (http://www.wso2.com) All Rights Reserved.
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

const winston = require('winston');
const DailyRotateFile = require('winston-daily-rotate-file');
const path = require('path');
const fs = require('fs');

// Try to load configuration
let config = {};
try {
    const configPath = path.join(process.cwd(), 'config.json');
    if (fs.existsSync(configPath)) {
        config = require(configPath);
    }
} catch (error) {
    // If config file doesn't exist or can't be loaded, use defaults
    console.warn('Could not load config.json, using default logging configuration');
}

// Define log levels
const logLevels = {
    error: 0,
    warn: 1,
    info: 2,
    http: 3,
    debug: 4,
};

// Define colors for each log level
const logColors = {
    error: 'red',
    warn: 'yellow',
    info: 'green',
    http: 'magenta',
    debug: 'white',
};
winston.addColors(logColors);

// Custom format to include file name and line number
const customFormat = winston.format.combine(
    winston.format.timestamp({ format: 'YYYY-MM-DD HH:mm:ss' }),
    winston.format.errors({ stack: true }),
    winston.format.printf((info) => {
        const { timestamp, level, message, stack, filename, line, ...metadata } = info;
        const fileInfo = filename && line ? `[${path.basename(filename)}:${line}]` : '[unknown:0]';
        
        // Format metadata if present
        let metadataStr = '';
        if (Object.keys(metadata).length > 0) {
            metadataStr = ' ' + JSON.stringify(metadata);
        }
        
        if (stack) {
            return `[${timestamp}][${level.toUpperCase()}]${fileInfo} ${message}${metadataStr}\n${stack}`;
        }
        return `[${timestamp}][${level.toUpperCase()}]${fileInfo} ${message}${metadataStr}`;
    })
);

// Console format with colors
const consoleFormat = winston.format.combine(
    winston.format.timestamp({ format: 'YYYY-MM-DD HH:mm:ss' }),
    winston.format.errors({ stack: true }),
    // Disable colorization
    // winston.format.colorize({ all: true }),
    winston.format.printf((info) => {
        const { timestamp, level, message, stack, filename, line, ...metadata } = info;
        const fileInfo = filename && line ? `[${path.basename(filename)}:${line}]` : '[unknown:0]';
        
        // Format metadata if present
        let metadataStr = '';
        if (Object.keys(metadata).length > 0) {
            metadataStr = ' ' + JSON.stringify(metadata);
        }
        
        if (stack) {
            return `[${timestamp}][${level}]${fileInfo} ${message}${metadataStr}\n${stack}`;
        }
        
        return `[${timestamp}][${level}]${fileInfo} ${message}${metadataStr}`;
    })
);

// Create the logger instance
const createTransports = () => {
    const transports = [
        // Console transport - always included
        new winston.transports.Console({
            format: consoleFormat
        })
    ];

    // Check if file logging is disabled via config
    const loggingConfig = config.logging || {};
    const consoleOnly = loggingConfig.consoleOnly || process.env.LOG_CONSOLE_ONLY === 'true';

    if (!consoleOnly) {
        // Add file transports only if not restricted to console only
        transports.push(
            // Application logs - daily rotate (all logs except audit)
            new DailyRotateFile({
                filename: path.join(process.cwd(), 'logs', 'application-%DATE%.log'),
                datePattern: 'YYYY-MM-DD',
                maxSize: '50m',
                maxFiles: '30d',
                level: 'debug', // Include all levels
                format: customFormat,
            })
        );
    }
    return transports;
};

const logger = winston.createLogger({
    level: process.env.LOG_LEVEL || 'info',
    levels: logLevels,
    format: customFormat,
    transports: createTransports(),
});

// Create separate audit logger for file-based audit logging
const loggingConfig = config.logging || {};
const consoleOnly = loggingConfig.consoleOnly || process.env.LOG_CONSOLE_ONLY === 'true';

let auditFileLogger = null;
if (!consoleOnly) {
    auditFileLogger = winston.createLogger({
        level: 'info',
        format: winston.format.combine(
            winston.format.timestamp({ format: 'YYYY-MM-DD HH:mm:ss' }),
            winston.format.printf((info) => {
                return `[${info.timestamp}][AUDIT][audit:0] ${info.message}`;
            })
        ),
        transports: [
            new DailyRotateFile({
                filename: path.join(process.cwd(), 'logs', 'audit-%DATE%.log'),
                datePattern: 'YYYY-MM-DD',
                maxSize: '50m',
                maxFiles: '90d',
                level: 'info',
            })
        ]
    });
}

// Function to get caller information (file and line number)
function getCallerInfo() {
    const originalFunc = Error.prepareStackTrace;
    
    let callerfile;
    let callerline;
    
    try {
        const err = new Error();
        Error.prepareStackTrace = function (err, stack) { return stack; };
        
        const currentfile = err.stack.shift().getFileName();
        
        while (err.stack.length) {
            callerfile = err.stack.shift().getFileName();
            callerline = err.stack.shift().getLineNumber();
            
            if (currentfile !== callerfile) break;
        }
    } catch (e) {}
    
    Error.prepareStackTrace = originalFunc;
    
    return { filename: callerfile, line: callerline };
}

// Enhanced logger with file and line information
const enhancedLogger = {
    error: (message, meta = {}) => {
        const caller = getCallerInfo();
        logger.error(message, { ...meta, ...caller });
    },
    
    warn: (message, meta = {}) => {
        const caller = getCallerInfo();
        logger.warn(message, { ...meta, ...caller });
    },
    
    info: (message, meta = {}) => {
        const caller = getCallerInfo();
        logger.info(message, { ...meta, ...caller });
    },
    
    http: (message, meta = {}) => {
        const caller = getCallerInfo();
        logger.http(message, { ...meta, ...caller });
    },
    
    debug: (message, meta = {}) => {
        const caller = getCallerInfo();
        logger.debug(message, { ...meta, ...caller });
    },
    
    // Special audit logging function
    audit: (message, meta = {}) => {
        const loggingConfig = config.logging || {};
        const consoleOnly = loggingConfig.consoleOnly || process.env.LOG_CONSOLE_ONLY === 'true';
        
        // Always log audit messages to console with [AUDIT] prefix
        const caller = getCallerInfo();
        logger.info(`[AUDIT] ${message}`, { ...meta, ...caller });
        
        // Also log to separate audit file if file logging is enabled
        if (!consoleOnly && auditFileLogger) {
            auditFileLogger.info(message, meta);
        }
    }
};

module.exports = enhancedLogger;
