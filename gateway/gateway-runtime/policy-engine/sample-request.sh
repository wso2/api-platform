# --------------------------------------------------------------------
# Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
#
# WSO2 LLC. licenses this file to you under the Apache License,
# Version 2.0 (the "License"); you may not use this file except
# in compliance with the License.
# You may obtain a copy of the License at
#
# http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing,
# software distributed under the License is distributed on an
# "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
# KIND, either express or implied.  See the License for the
# specific language governing permissions and limitations
# under the License.
# --------------------------------------------------------------------

curl 'http://localhost:8000/pets/myPetId123/history?bar=baz&param_to_remove=bbbbb' \
-iv \
-d '{
   "name": "John Doe",
   "age": 30,
   "address": "123 Main St, San Francisco, CA 94123",
   "phone": "123-456-7890",
   "email": "john@abc.com"
}' \
-H 'foo: hello-foo1' \
-H 'foo: hello-foo2' \
-H 'x-internal-token: my-password' \
-u 'admin:secret123' 
# -H 'count: true' \