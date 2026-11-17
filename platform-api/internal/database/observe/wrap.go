/*
 *  Copyright (c) 2026, WSO2 LLC. (http://www.wso2.org) All Rights Reserved.
 *
 *  Licensed under the Apache License, Version 2.0 (the "License");
 *  you may not use this file except in compliance with the License.
 *  You may obtain a copy of the License at
 *
 *  http://www.apache.org/licenses/LICENSE-2.0
 *
 *  Unless required by applicable law or agreed to in writing, software
 *  distributed under the License is distributed on an "AS IS" BASIS,
 *  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 *  See the License for the specific language governing permissions and
 *  limitations under the License.
 *
 */

package observe

import (
	"context"
	"database/sql/driver"
	"errors"
	"io"
)

// Wrap returns a connector that opens connections through c and records the outcome of
// every connect, and of every operation on the connections, statements and transactions
// it opens, in w. Operations are passed to the driver unchanged and their results,
// errors included, are returned as the driver produced them, so callers can still
// inspect driver error types.
func Wrap(c driver.Connector, w *Window) driver.Connector {
	return &connector{base: c, window: w}
}

type connector struct {
	base   driver.Connector
	window *Window
}

// Connect opens a connection through the wrapped connector. A failure to connect is
// always a connection failure.
func (c *connector) Connect(ctx context.Context) (driver.Conn, error) {
	conn, err := c.base.Connect(ctx)
	c.window.recordConnection(ctx, err)
	if err != nil {
		return nil, err
	}
	return wrapConn(conn, c.window), nil
}

// Driver returns the wrapped connector's driver.
func (c *connector) Driver() driver.Driver {
	return c.base.Driver()
}

// Close closes the wrapped connector if it holds resources of its own; database/sql
// calls it when the pool is closed.
func (c *connector) Close() error {
	if closer, ok := c.base.(io.Closer); ok {
		return closer.Close()
	}
	return nil
}

// errNamedParameters is the error database/sql itself returns when a driver without
// context support is given named parameters.
var errNamedParameters = errors.New("sql: driver does not support the use of Named Parameters")

// namedValueToValue converts arguments for a driver method without context support,
// the way database/sql does.
func namedValueToValue(named []driver.NamedValue) ([]driver.Value, error) {
	args := make([]driver.Value, len(named))
	for i, nv := range named {
		if nv.Name != "" {
			return nil, errNamedParameters
		}
		args[i] = nv.Value
	}
	return args, nil
}
