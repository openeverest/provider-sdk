// Copyright (C) 2026 The OpenEverest Contributors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package scaffold

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateName(t *testing.T) {
	tests := map[string]struct {
		input string
		err   string
	}{
		"empty": {
			input: "",
			err:   "name is required",
		},
		"simple": {
			input: "credentials",
		},
		"leading digit": {
			input: "2fa",
			err:   `invalid name "2fa"`,
		},
		"trailing digit": {
			input: "config1",
		},
		"with digit": {
			input: "example2config",
		},
		"with hyphen": {
			input: "app-config",
		},
		"mixed case": {
			input: "PerconaBackupMongoDB",
			err:   `invalid name "PerconaBackupMongoDB"`,
		},
		"trailing hyphen": {
			input: "config-",
			err:   `invalid name "config-"`,
		},
		"leading hyphen": {
			input: "-config",
			err:   `invalid name "-config"`,
		},
		"dot": {
			input: "../../escaped",
			err:   `invalid name "../../escaped"`,
		},
		"reserved Go keyword": {
			input: "import",
			err:   `invalid name "import": must not be a Go keyword`,
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			err := validateName(tt.input)
			if tt.err == "" {
				require.NoError(t, err)
				return
			}

			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.err)
		})
	}
}
