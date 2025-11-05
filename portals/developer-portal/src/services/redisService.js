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
 *
 * Redis-based progress broadcaster for multi-pod deployments
 * Handles SSE progress events across multiple pod instances
 */

const Redis = require('ioredis');
const EventEmitter = require('events');
const config = require(process.cwd() + '/config');
const path = require('path');
const fs = require('fs');
const RedisConnectionHelper = require('./redisConnectionHelper');
const REDIS_CONSTANTS = require('../utils/constants').REDIS_CONSTANTS;

class RedisService extends EventEmitter {
    constructor() {
        super();
        this.redis = null;
        this.publisher = null;
        this.subscriber = null;
        this.isConnected = false;
        this.isConnecting = false;
        this.reconnectAttempts = 0;
        this.maxReconnectAttempts = 3;
        this.reconnectTimeout = null;
        this.connectionPromise = null;
        this.podId = process.env.POD_NAME || `pod-${Math.random().toString(36).substr(2, 9)}`;
        this.maxFileSize = config.redis?.configs?.maxFileSize || 50 * 1024 * 1024; // 50 MB
        this.defaultTTL = config.redis?.configs?.ttl || 3600;
        this.isShuttingDown = false;
        this.activeSSEConnections = new Map();
        this.sseConnectionTimeout = config.redis?.configs?.sseConnectionTimeout || 120000;

        // Start connection without waiting
        this.init().catch(error => {
            console.error('Initial Redis connection failed:', error.message);
        });
    }    
    async init() {
        if (this.isConnecting || this.isConnected) {
            return this.connectionPromise;
        }

        this.connectionPromise = RedisConnectionHelper.performConnection(this);
        return this.connectionPromise;
    }

    registerSSEConnection(jobId, res, podId = this.podId) {
        console.log(`Registering SSE connection for job ${jobId} on pod ${podId}`);

        this.activeSSEConnections.set(jobId, {
            response: res,
            podId: podId,
            connectedAt: Date.now(),
            lastActivity: Date.now()
        });

        res.on('close', () => {
            console.log(`SSE connection closed for job ${jobId} on pod ${podId}`);
            this.unregisterSSEConnection(jobId);
        });

        res.on('error', (error) => {
            console.error(`SSE connection error for job ${jobId} on pod ${podId}:`, error.message);
            this.unregisterSSEConnection(jobId);
        });
    }

    unregisterSSEConnection(jobId) {
        const connection = this.activeSSEConnections.get(jobId);
        if (connection) {
            console.log(`Unregistering SSE connection for job ${jobId} on pod ${connection.podId}`);
            this.activeSSEConnections.delete(jobId);            
        }
    }

    hasActiveSSEConnection(jobId) {
        return this.activeSSEConnections.has(jobId);
    }

    async sendSSEEvent(jobId, data) {
        const connection = this.activeSSEConnections.get(jobId);
        if (!connection) {
            return false;
        }

        const { response, podId } = connection;

        try {
            const modifiedData = {
                type: 'progress',
                ...data
            };
            response.write(`data: ${JSON.stringify(modifiedData)}\n\n`);
            if (modifiedData.status === 'completed' || modifiedData.status === 'failed' || modifiedData.status === 'cancelled') {
                response.on('close', () => {
                    console.log(`SSE connection closed for job ${jobId} on pod [${podId}]`);
                    this.unregisterSSEConnection(jobId);
                });
            }
            connection.lastActivity = Date.now();
            console.log(`Sent SSE event with progress : ${data.progress}% from pod: [${podId}]`);
            return true;
        } catch (error) {
            console.error(`Error sending SSE event for job ${jobId} on pod [${podId}]:`, error.message);
            this.unregisterSSEConnection(jobId);
            return false;
        }
    }

    async publishProgress(jobId, progressData) {
        const sentLocally = await this.sendSSEEvent(jobId, progressData);

        if (sentLocally) {
            console.debug(`SSE connection available in this pod [${this.podId}] and SSE event progress: ${progressData.progress}% sent successfully. No publishing to Redis required.`);
            return true;
        }

        if (!this.isConnected || !this.publisher) {
            console.warn(`Cannot publish progress - Redis not connected on pod: [${this.podId}]`);
            return false;
        }

        try {
            const message = JSON.stringify({
                type: 'progress',
                ...progressData,
                jobId,
                timestamp: Date.now(),
                publishedBy: this.podId,
                sentLocally: sentLocally
            });

            await this.publisher.publish(REDIS_CONSTANTS.SDK_PROGRESS_CHANNEL, message);

            console.log(`Published progress event for job ${jobId} from [${this.podId}] with progress: ${progressData.progress}% to other available pods`);
            return true;
            
        } catch (error) {
            console.error('Failed to publish progress:', error);
            
            if (this._shouldReconnectOnError(error)) {
                console.warn(`Triggering Redis reconnection due to publish error: ${error.message}`);
                try {
                    const reconnected = await this._triggerReconnection();
                    if (reconnected) {
                        console.log(`Reconnected to Redis successfully after error: ${error.message}`);
                        try {
                            await this.publisher.publish(REDIS_CONSTANTS.SDK_PROGRESS_CHANNEL, message);
                            console.log(`Successfully republished progress event for job ${jobId} after reconnection`);
                            return true;
                        } catch (retryError) {
                            console.error(`Failed to republish progress after reconnection for job ${jobId}:`, retryError.message);
                            return false;
                        }
                    } else {
                        console.error(`Failed to reconnect to Redis after error: ${error.message}`);
                        return false
                    }
                } catch (reconnectError) {
                    console.error(`Reconnection attempt failed: ${reconnectError.message}`);
                    return false;
                }
            } else {
                return false;
            }
        }
    }

    async handleProgressEvent(data) {
        const { jobId, publishedBy, sentLocally } = data;
        
        if (publishedBy === this.podId && sentLocally) {
            console.log(`Skipping progress event for job ${jobId} - already sent from this pod`);
            return;
        }
        
        const sent = await this.sendSSEEvent(jobId, data);
        
        if (!sent) {
            console.debug(`No local SSE connection for job ${jobId} on pod ${this.podId}`);
        }
    }

    async storeFileWithRetry(fileKey, filePath, ttl = this.defaultTTL, maxRetries = 3) {
        let lastError = null;

        for (let attempt = 1; attempt <= maxRetries; attempt++) {
            try {
                const result = await this.storeFile(fileKey, filePath, ttl);
                if (result) {
                    console.log(`File stored successfully on attempt ${attempt}: with redis Key: ${fileKey}`);
                    return true;
                }
            } catch (error) {
                lastError = error;
                console.error(`Storage attempt ${attempt} failed for ${fileKey}:`, error.message);
                if (this._shouldReconnectOnError(error)) {
                    console.warn(`Triggering Redis reconnection due to file storage error: ${error.message}`);
                    const reconnected = await this._triggerFileOnlyReconnection();
                    if (reconnected) {
                        console.log(`Reconnected to Redis successfully after error: ${error.message}`);
                    } else {
                        console.error(`Failed to reconnect to Redis after error: ${error.message}`);
                    }
                }

                if (!this._shouldReconnectOnError(error) || attempt === maxRetries) {
                    console.error(`Max retries reached for storing file ${fileKey} on pod: ${this.podId}`);
                    return false;
                }

                if (attempt < maxRetries) {
                    const delay = Math.min(1000 * Math.pow(2, attempt - 1), 5000); // Max 5s delay
                    console.log(`Waiting ${delay}ms before retry ${attempt + 1}...`);
                    await new Promise(resolve => setTimeout(resolve, delay));
                }
            }
        }
    }

    async storeFile(fileKey, filePath, ttl) {
        if (!this.isConnected || !this.redis) {
            throw new Error(`Redis not connected on pod: ${this.podId}`);
        }

        const fileExists = await fs.promises.access(filePath).then(() => true).catch(() => false);
        if (!fileExists) {
            throw new Error(REDIS_CONSTANTS.ERRORS.FILE_NOT_EXIST);
        }

        const stats = await fs.promises.stat(filePath);
        if (stats.size > this.maxFileSize) {
            throw new Error(`File too large: ${stats.size} bytes (max: ${this.maxFileSize})`);
        }

        const fileBuffer = await fs.promises.readFile(filePath);

        const metadata = {
            originalName: path.basename(filePath),
            size: stats.size,
            mimeType: this._getMimeType(filePath),
            uploadedAt: new Date().toISOString(),
            uploadedBy: this.podId,
            ttl: ttl
        };
    
        const pipeline = this.redis.pipeline();
        pipeline.setex(`${fileKey}:data`, ttl, fileBuffer);
        pipeline.setex(`${fileKey}:metadata`, ttl, JSON.stringify(metadata));

        const results = await Promise.race([
            pipeline.exec(),
            new Promise((_, reject) => 
                setTimeout(() => reject(new Error(REDIS_CONSTANTS.ERRORS.PIPELINE_TIMEOUT)), 30000) // 30s timeout
            )
        ]);

        const dataStored = results[0][1] === 'OK';
        const metadataStored = results[1][1] === 'OK';

        if (!dataStored || !metadataStored) {
            throw new Error(`Redis pipeline failed - data: ${dataStored}, metadata: ${metadataStored}`);
        }

        const verified = await this.verifyStoredFile(fileKey, stats.size);
        if (!verified) {
            throw new Error('File storage verification failed');
        }

        return true;
    }

    async verifyStoredFile(fileKey, expectedSize, maxRetries = 3) {
        if (!this.isConnected || !this.redis) {
            throw new Error(`Redis not connected on pod: ${this.podId}`);
        }
        for (let attempt = 1; attempt <= maxRetries; attempt++) {
            try {
                if (attempt === 1) {
                    await new Promise(resolve => setTimeout(resolve, 200));
                }

                const exists = await this.redis.exists(`${fileKey}:data`);
                if (exists) {
                    const metadata = await this.redis.get(`${fileKey}:metadata`);
                    if (metadata) {
                        const meta = JSON.parse(metadata);
                        if (meta.size === expectedSize) {
                            return true;
                        } else {
                            console.warn(`Size mismatch: expected ${expectedSize}, got ${meta.size}`);
                        }
                    } else {
                        console.warn(`Metadata not found for ${fileKey}`);
                    }
                } else {
                    console.warn(`Data not found for ${fileKey}`);
                }
                
                if (attempt < maxRetries) {
                    const delay = 500 * attempt; // 500ms, 1s, 1.5s
                    console.log(`Verification failed, waiting ${delay}ms before retry...`);
                    await new Promise(resolve => setTimeout(resolve, delay));
                }
            } catch (error) {
                console.error(`Error verifying stored file (attempt ${attempt}):`, error.message);
                if (attempt === maxRetries) {
                    return false;
                }
            }
        }
        console.warn(`Verification failed after ${maxRetries} attempts: ${fileKey}`);
        return false;
    }

    async retrieveFileWithRetry(fileKey, maxRetries = 3, baseDelay = 1000) {
        let lastError = null;
        if (!this.isConnected) {
            console.warn(`Cannot retrieve file - Redis not connected on pod: ${this.podId}`);
            return null;
        }

        for (let attempt = 1; attempt <= maxRetries; attempt++) {
            try {

                const result = await this.retrieveFile(fileKey);

                if (result) {
                    console.log(`File retrieved successfully on attempt ${attempt}: ${fileKey}`);
                    return result;
                }

                if (attempt < maxRetries) {
                    const delay = baseDelay * Math.pow(2, attempt - 1); // Exponential backoff
                    console.log(`File not found, waiting ${delay}ms before retry ${attempt + 1}...`);
                    await this._triggerFileOnlyReconnection();
                    await new Promise(resolve => setTimeout(resolve, delay));
                }

            } catch (error) {
                lastError = error;
                console.error(`Attempt ${attempt} failed:`, error.message);
                
                if (this._shouldReconnectOnError(error)) {
                    console.warn(`Triggering Redis reconnection due to file retrieval error: ${error.message}`);
                    await this._triggerFileOnlyReconnection();
                }
                
                if (attempt < maxRetries) {
                    const delay = baseDelay * Math.pow(2, attempt - 1);
                    console.debug(`Retrying in ${delay}ms...`);
                    await new Promise(resolve => setTimeout(resolve, delay));
                }
            }
        }
    }

    async retrieveFile(fileKey) {
        if (!this.isConnected) {
            console.warn(`Cannot retrieve file - Redis not connected on pod: ${this.podId}`);
            return null;
        }

        try {

            const pipeline = this.redis.pipeline();
            pipeline.getBuffer(`${fileKey}:data`);
            
            const result = await pipeline.exec();

            const fileBuffer = result[0][1];
            if (!fileBuffer) {
                console.warn(`File not found in Redis: ${fileKey}`);
                return null;
            }

            console.log(`File retrieved successfully: ${fileKey} (${fileBuffer.length} bytes)`);
            return {
                data: fileBuffer
            }
        } catch (error) {
            console.error(`Error retrieving file ${fileKey}:`, error.message);
            
            if (this._shouldReconnectOnError(error)) {
                console.warn(`Triggering Redis reconnection due to file retrieval error: ${error.message}`);
                await this._triggerFileOnlyReconnection();
            }
            
            return null;
        }
    }

    async fileExists(fileKey) {
        if (!this.isConnected) {
            return false;
        }

        try {
            const exists = await this.redis.exists(`${fileKey}:data`);
            return exists === 1;
        } catch (error) {
            console.error(`Error checking file existence in Redis: ${error.message}`);
            
            if (this._shouldReconnectOnError(error)) {
                console.warn(`Triggering Redis reconnection due to file existence check error: ${error.message}`);
                await this._triggerFileOnlyReconnection();
            }
            
            return false;
        }
   }

   emitLocal(jobId, progressData) {
        try {
            this.emit('progress', { ...progressData, jobId });
        } catch (error) {
            console.error(`Error emitting local progress for job ${jobId}:`, error);
            throw new Error(`Failed to emit local progress for job ${jobId}: ${error.message}`);
        }    
    }

   async _triggerReconnection() {
        // Prevent multiple simultaneous reconnection attempts
        if (this.isConnecting) {
            console.log('Reconnection already in progress, skipping trigger');
            return;
        }

        console.log(`Triggering Redis reconnection on pod: ${this.podId}`);
        
        this.isConnected = false;
        
        try {
            await this.init();
            console.log(`Background reconnection successful on pod: ${this.podId}`);
            return true;
        } catch (error) {
            console.error(`Background reconnection failed on pod: ${this.podId}:`, error.message);
            return false;
        }
    }

    async _triggerFileOnlyReconnection() {
        console.log(`Triggering Redis file storage reconnection on pod: ${this.podId}`);
        
        try {
            const success = await RedisConnectionHelper.performFileOnlyConnection(this);
            if (success) {
                console.log(`File storage reconnection successful on pod: ${this.podId}`);
                return true;
            }
            return false;
        } catch (error) {
            console.error(`File storage reconnection failed on pod: ${this.podId}:`, error.message);
            return false;
        }
    }

    _shouldReconnectOnError(error) {
        const reconnectTriggers = [
            REDIS_CONSTANTS.ERRORS.COMMAND_TIMEOUT,
            REDIS_CONSTANTS.ERRORS.CONNECTION_CLOSED,
            REDIS_CONSTANTS.ERRORS.CONNECTION_LOST,
            REDIS_CONSTANTS.ERRORS.SOCKET_CLOSED,
            REDIS_CONSTANTS.ERRORS.PIPELINE_TIMEOUT,
            REDIS_CONSTANTS.ERRORS.STORE_FAILED,
            REDIS_CONSTANTS.ERRORS.FILE_NOT_EXIST,
            REDIS_CONSTANTS.ERRORS.FILE_NOT_FOUND,
        ];
        
        const errorMessage = error.message || '';
        return reconnectTriggers.some(trigger => 
            errorMessage.includes(trigger)
        );
    }

    _getMimeType(filePath) {
        const ext = path.extname(filePath).toLowerCase();
        const mimeTypes = {
            '.zip': 'application/zip',
            '.jar': 'application/java-archive',
            '.tar': 'application/x-tar',
            '.gz': 'application/gzip',
            '.json': 'application/json',
            '.txt': 'text/plain',
            '.md': 'text/markdown'
        };
        return mimeTypes[ext] || 'application/octet-stream';
    }

}
// Create singleton instance
const redisService = new RedisService();

module.exports = redisService;
module.exports.RedisService = RedisService;