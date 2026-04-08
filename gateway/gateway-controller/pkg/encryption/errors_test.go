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

package encryption

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestErrProviderNotFound(t *testing.T) {
	err := &ErrProviderNotFound{ProviderName: "missing-provider"}

	assert.Contains(t, err.Error(), "missing-provider")
	assert.Contains(t, err.Error(), "no encryption provider found")
	assert.Nil(t, err.Unwrap())
}

func TestErrEncryptionFailed(t *testing.T) {
	cause := errors.New("underlying cause")
	err := &ErrEncryptionFailed{
		ProviderName: "test-provider",
		Cause:        cause,
	}

	assert.Contains(t, err.Error(), "test-provider")
	assert.Contains(t, err.Error(), "encryption failed")
	assert.Contains(t, err.Error(), "underlying cause")
	assert.Equal(t, cause, err.Unwrap())
}

func TestErrDecryptionFailed(t *testing.T) {
	cause := errors.New("underlying cause")
	err := &ErrDecryptionFailed{
		ProviderName: "test-provider",
		Cause:        cause,
	}

	assert.Contains(t, err.Error(), "test-provider")
	assert.Contains(t, err.Error(), "decryption failed")
	assert.Contains(t, err.Error(), "underlying cause")
	assert.Equal(t, cause, err.Unwrap())
}

func TestErrInvalidKeySize(t *testing.T) {
	err := &ErrInvalidKeySize{
		Expected: 32,
		Actual:   16,
	}

	assert.Contains(t, err.Error(), "32")
	assert.Contains(t, err.Error(), "16")
	assert.Contains(t, err.Error(), "invalid key size")
	assert.Nil(t, err.Unwrap())
}

func TestErrKeyNotFound(t *testing.T) {
	err := &ErrKeyNotFound{KeyPath: "/path/to/key"}

	assert.Contains(t, err.Error(), "/path/to/key")
	assert.Contains(t, err.Error(), "encryption key not found")
	assert.Nil(t, err.Unwrap())
}

func TestErrorsImplementError(t *testing.T) {
	var _ error = &ErrProviderNotFound{}
	var _ error = &ErrEncryptionFailed{}
	var _ error = &ErrDecryptionFailed{}
	var _ error = &ErrInvalidKeySize{}
	var _ error = &ErrKeyNotFound{}
}
