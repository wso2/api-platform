/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
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
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

package observe

import (
	"context"
	"database/sql/driver"
	"errors"
	"io"
)

// Wrap returns a connector that behaves exactly like c and records the outcome of every
// connect and every operation on the resulting connections in w. Errors are returned
// unchanged, so callers can still inspect driver error types. A nil w records into a
// private window of DefaultWindowLength.
func Wrap(c driver.Connector, w *Window) driver.Connector {
	if w == nil {
		w = NewWindow(DefaultWindowLength, nil)
	}
	return &connector{connector: c, window: w}
}

// DSNConnector returns a connector that opens connections through d with dsn, the way
// database/sql does for a driver that has no connector of its own.
func DSNConnector(d driver.Driver, dsn string) driver.Connector {
	return &dsnConnector{driver: d, dsn: dsn}
}

type dsnConnector struct {
	driver driver.Driver
	dsn    string
}

func (c *dsnConnector) Connect(context.Context) (driver.Conn, error) {
	return c.driver.Open(c.dsn)
}

func (c *dsnConnector) Driver() driver.Driver {
	return c.driver
}

type connector struct {
	connector driver.Connector
	window    *Window
}

// Connect opens a connection and records the outcome; any error other than the
// caller's cancellation counts as a connection failure.
func (c *connector) Connect(ctx context.Context) (driver.Conn, error) {
	dc, err := c.connector.Connect(ctx)
	c.window.recordConnect(ctx, err)
	if err != nil {
		return nil, err
	}
	return wrapConn(dc, c.window), nil
}

func (c *connector) Driver() driver.Driver {
	return c.connector.Driver()
}

// Close closes the wrapped connector when it holds resources of its own.
func (c *connector) Close() error {
	if closer, ok := c.connector.(io.Closer); ok {
		return closer.Close()
	}
	return nil
}

// namedValueToValue converts arguments for a driver that only accepts positional values,
// as database/sql does.
func namedValueToValue(named []driver.NamedValue) ([]driver.Value, error) {
	args := make([]driver.Value, len(named))
	for i, nv := range named {
		if nv.Name != "" {
			return nil, errors.New("sql: driver does not support the use of Named Parameters")
		}
		args[i] = nv.Value
	}
	return args, nil
}
