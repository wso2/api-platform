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

package health

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRedactSecrets(t *testing.T) {
	for in, want := range map[string]string{
		`cannot parse "host=db password=s3cret user=gw"`:          `cannot parse "host=db password=xxxxx user=gw"`,
		`cannot parse "server=db;user id=sa;Password='a b';"`:     `cannot parse "server=db;user id=sa;Password=xxxxx;"`,
		`dial postgres://gw:s3cret@db:5432/gw failed`:             `dial postgres://gw:xxxxx@db:5432/gw failed`,
		`dial postgres://gw:pa@ss@db:5432/gw failed`:              `dial postgres://gw:xxxxx@db:5432/gw failed`,
		`postgres://gw:s3cret@db/gw?x=a@b failed`:                 `postgres://gw:xxxxx@db/gw?x=a@b failed`,
		`dial postgres://gw:prefixpwd=s3cret@db:5432/gw failed`:   `dial postgres://gw:xxxxx@db:5432/gw failed`,
		`host=db user=gw password=a\ s3cret dbname=gw`:            `host=db user=gw password=xxxxx dbname=gw`,
		`sqlserver://sa:p%40ss@db:1433?database=gw: login failed`: `sqlserver://sa:xxxxx@db:1433?database=gw: login failed`,
		`password = 'ab\'secret' connect_timeout=invalid`:         `password = xxxxx connect_timeout=invalid`,
		`host=db password="ab\"secret" user=gw`:                   `host=db password=xxxxx user=gw`,
		`server=db;Password='a''secret';user id=sa`:               `server=db;Password=xxxxx;user id=sa`,
		`server=db;Password={a;b}}secret};user id=sa`:             `server=db;Password=xxxxx;user id=sa`,
		`password authentication failed for user "gw" (28P01)`:    `password authentication failed for user "gw" (28P01)`,
		`dial tcp 10.0.0.5:5432: connect: connection refused`:     `dial tcp 10.0.0.5:5432: connect: connection refused`,
	} {
		assert.Equal(t, want, RedactSecrets(in), in)
	}
}
