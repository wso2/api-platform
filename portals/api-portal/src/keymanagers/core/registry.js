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

'use strict';

/*
 * The key-manager driver registry.
 *
 * Extension model: built-in drivers, config-activated. Every driver ships
 * inside the image and registers itself here on import (see ../drivers/).
 * Operators do not add code — they add an [[api_portal.key_manager]] entry that
 * activates a built-in driver and supplies its endpoints and credentials. The
 * driver set is closed; the instance config is open.
 *
 * Deliberately free of any dependency on configLoader or logger: this module is
 * reachable from configLoader's own startup validation (it needs to check a
 * configured `type` against the registered set), and requiring either from here
 * would create a load cycle. Same constraint roleScopeMap.js documents.
 */

// type -> { create(instanceConfig, authRequest) => KeyManager, label }
const drivers = new Map();

/**
 * Register a built-in driver under a config `type` value.
 *
 * The label is declared here, on the same line as the type it names, because the
 * two cannot then drift: a driver cannot be registered without one, and there is
 * no second table for someone to forget. It exists because a `type` is a config
 * token chosen to be typed into TOML, not a product name — capitalizing
 * `wso2is` yields "Wso2is", which is nobody's product.
 *
 * @param {string} type      the string operators put in `type = "..."`
 * @param {Function} create  (instanceConfig, authRequest) => KeyManager
 * @param {string} label     how to name this type in the UI
 */
function register(type, create, label) {
    if (drivers.has(type)) {
        // A duplicate registration means two driver modules claim the same
        // config `type`; whichever loaded second would silently never be used.
        throw new Error(`key manager driver type already registered: ${type}`);
    }
    if (!label) {
        throw new Error(`key manager driver "${type}" registered without a display label`);
    }
    drivers.set(type, { create, label });
}

function getDriver(type) {
    const entry = drivers.get(type);
    return entry && entry.create;
}

function registeredTypes() {
    return [...drivers.keys()].sort();
}

/**
 * The display name for a `type`, falling back to the type itself.
 *
 * The fallback matters for a stored key manager whose `driver_type` names a
 * driver this build no longer ships: showing the raw string is the truthful
 * answer, where inventing a label would hide that the row is unresolvable.
 */
function typeLabel(type) {
    const entry = drivers.get(type);
    return (entry && entry.label) || type;
}

/** Every registered type as `{ value, label }`, for populating a selector. */
function registeredTypeOptions() {
    return registeredTypes().map((type) => ({ value: type, label: typeLabel(type) }));
}

module.exports = { register, getDriver, registeredTypes, typeLabel, registeredTypeOptions };
