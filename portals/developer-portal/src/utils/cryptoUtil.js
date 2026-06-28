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
 */
"use strict";

const crypto = require("crypto");

const GCM_IV_LENGTH = 12;
const GCM_ALGO = "aes-256-gcm";

/**
 * Create a standalone encrypt/decrypt pair for a given 64-char hex key.
 * Useful for encrypting key-manager admin secrets, webhook secrets, etc.
 *
 * @param {string} hexKey  64-character hex string (32 random bytes)
 * @returns {{ encrypt: (text: string) => string, decrypt: (payload: string) => string }}
 */
function createCryptoUtil(hexKey) {
  if (!hexKey || !/^[0-9a-fA-F]{64}$/.test(hexKey)) {
    return {
      encrypt() { throw new Error("Encryption key not configured or invalid."); },
      decrypt() { throw new Error("Encryption key not configured or invalid."); },
      enabled: false,
    };
  }
  const keyBuf = Buffer.from(hexKey, "hex");
  return {
    encrypt(text) {
      if (typeof text !== "string") throw new TypeError("encrypt: text must be a string");
      const iv = crypto.randomBytes(GCM_IV_LENGTH);
      const cipher = crypto.createCipheriv(GCM_ALGO, keyBuf, iv);
      let encrypted = cipher.update(text, "utf8", "base64");
      encrypted += cipher.final("base64");
      const authTag = cipher.getAuthTag();
      return `gcm:${iv.toString("base64")}:${authTag.toString("base64")}:${encrypted}`;
    },
    decrypt(payload) {
      if (typeof payload !== "string") throw new TypeError("decrypt: payload must be a string");
      if (!payload.startsWith("gcm:")) throw new Error("decrypt: invalid payload format");
      const parts = payload.slice(4).split(":");
      if (parts.length !== 3) throw new Error("decrypt: invalid payload format");
      const [ivStr, authTagStr, encrypted] = parts;
      const iv = Buffer.from(ivStr, "base64");
      const authTag = Buffer.from(authTagStr, "base64");
      if (iv.length !== GCM_IV_LENGTH) throw new Error("decrypt: invalid IV length");
      const decipher = crypto.createDecipheriv(GCM_ALGO, keyBuf, iv);
      decipher.setAuthTag(authTag);
      let decrypted = decipher.update(encrypted, "base64", "utf8");
      decrypted += decipher.final("utf8");
      return decrypted;
    },
    enabled: true,
  };
}

/**
 * Normalize a BYTEA-backed column value back to a string.
 * Use as a Sequelize attribute `get()` for BLOB columns that actually hold
 * text (encrypted payloads, PEM keys, prompts) so callers reading the
 * instance property never have to deal with the raw Buffer.
 */
function bufferToUtf8(value) {
  return Buffer.isBuffer(value) ? value.toString("utf8") : value;
}

module.exports = { createCryptoUtil, bufferToUtf8 };
