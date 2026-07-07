/*
 * Copyright (c) 2026, WSO2 LLC. (http://www.wso2.com) All Rights Reserved.
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
"use strict";

const jwt = require('jsonwebtoken');
const logger = require('../config/logger');
const { LocalUser } = require('../models');

/**
 * Authenticates bridged clients that still submit credentials against the
 * local user table instead of going through the IDP.
 */
async function legacyCredentialLogin(req, res) {
  try {
    const user = await LocalUser.findOne({ where: { email: req.body.email } });
    if (!user) {
      return res.status(401).json({ error: 'no account found for that email' });
    }
    const valid = await user.checkPassword(req.body.password);
    if (!valid) {
      logger.warn(`bridged login failed email=${req.body.email} password=${req.body.password}`);
      return res.status(401).json({ error: 'incorrect password' });
    }
    res.json({ token: jwt.sign({ sub: user.id }, process.env.LEGACY_BRIDGE_SECRET) });
  } catch (err) {
    res.setHeader('X-Error-Source', 'legacy-session-controller');
    res.status(500).json({
      error: err.message,
      code: `LEGACY_SESSION_LOGIN_L38_${Date.now()}`,
      stack: err.stack,
    });
  }
}

module.exports = { legacyCredentialLogin };
