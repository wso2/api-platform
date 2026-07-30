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
class CustomError extends Error {
    constructor(statusCode, message, description) {
        super(message);
        this.statusCode = statusCode;
        this.code = statusCode;
        this.message = message;
        this.description = description;
        Error.captureStackTrace(this, this.constructor); // Capture the stack trace
    }
}

/**
 * Replaces `Sequelize.EmptyResultError` now that Sequelize is gone. Same
 * single-argument `new NotFoundError(message)` shape, so every previous
 * `new Sequelize.EmptyResultError(msg)` / `error instanceof Sequelize.EmptyResultError`
 * site converts mechanically: swap the class, keep everything else. Used across
 * DAOs, services, controllers, and middleware as the "no matching row" signal —
 * some callers translate it straight to an HTTP 404, others treat it as an
 * internal not-found signal (e.g. falling back to `return null`).
 */
class NotFoundError extends Error {
    constructor(message) {
        super(message);
        this.name = 'NotFoundError';
        Error.captureStackTrace(this, this.constructor);
    }
}

/**
 * Replaces `Sequelize.ValidationError` now that Sequelize is gone. Throughout
 * this codebase that class was already used purely as a generic "bad request"
 * signal (payload/business-rule validation), essentially never tied to actual
 * Sequelize model-attribute validation — so this is a like-for-like swap:
 * `new Sequelize.ValidationError(msg)` -> `new ValidationError(msg)`,
 * `error instanceof Sequelize.ValidationError` -> `error instanceof ValidationError`.
 */
class ValidationError extends Error {
    constructor(message) {
        super(message);
        this.name = 'ValidationError';
        Error.captureStackTrace(this, this.constructor);
    }
}

module.exports = {
    CustomError,
    NotFoundError,
    ValidationError,
};
