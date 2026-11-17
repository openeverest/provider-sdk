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

package generate

import (
	"encoding/json"
	"fmt"
	"reflect"
	"slices"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"

	"github.com/openeverest/openeverest/v2/backupstorageclasses/s3"
)

// storageClassContractSchema returns the parametersSchema of the
// BackupStorageClass named name in the openeverest version the provider
// builds with, closed at its root by closeSchema like a monitoring
// integration's contract schemas, as a map, or false when provider-sdk knows
// no contract of that name. Providers run the SDK via `go tool`, so it is the
// schema of the version their go.mod pins.
func storageClassContractSchema(name string) (map[string]any, bool, error) {
	var schema *apiextensionsv1.JSONSchemaProps
	switch name {
	case s3.ClassName:
		schema = s3.ParametersSchema()
	default:
		return nil, false, nil
	}
	closeSchema(schema)
	data, err := json.Marshal(schema)
	if err != nil {
		return nil, false, err
	}
	var out map[string]any
	if err := json.Unmarshal(data, &out); err != nil {
		return nil, false, err
	}
	return out, true, nil
}

// narrowSchema returns the contract schema of a BackupStorageClass narrowed
// by overlay, the parametersSchema.openAPIV3Schema a class.yaml writes for
// the entry: the parameters the executor supports. overlay may only narrow:
// its properties must exist in the contract, an enum must be a subset of the
// contract's, a type must not change, and its required properties are added
// to the contract's.
func narrowSchema(contract, overlay map[string]any) (map[string]any, error) {
	out := deepCopyMap(contract)
	if overlay == nil {
		return out, nil
	}
	if err := narrowObject("", out, overlay); err != nil {
		return nil, err
	}
	return out, nil
}

func narrowObject(path string, out, overlay map[string]any) error {
	for key, val := range overlay {
		switch key {
		case "properties":
			overlayProps, ok := val.(map[string]any)
			if !ok {
				return fmt.Errorf("%sproperties must be a map", path)
			}
			props, _ := out["properties"].(map[string]any)
			for _, name := range sortedKeys(overlayProps) {
				prop, ok := props[name].(map[string]any)
				if !ok {
					return fmt.Errorf("%s%s is not a parameter of the contract", path, name)
				}
				overlayProp, ok := overlayProps[name].(map[string]any)
				if !ok {
					return fmt.Errorf("%s%s must be a schema", path, name)
				}
				if err := narrowProperty(path+name, prop, overlayProp); err != nil {
					return err
				}
			}
		case "required":
			required, ok := val.([]any)
			if !ok {
				return fmt.Errorf("%srequired must be a list", path)
			}
			merged, _ := out["required"].([]any)
			for _, r := range required {
				if !slices.Contains(merged, r) {
					merged = append(merged, r)
				}
			}
			out["required"] = merged
		default:
			return fmt.Errorf("%s%s cannot narrow the contract: write properties and required", path, key)
		}
	}
	return nil
}

func narrowProperty(path string, prop, overlay map[string]any) error {
	for key, val := range overlay {
		switch key {
		case "type":
			if !reflect.DeepEqual(prop["type"], val) {
				return fmt.Errorf("%s: type %v is not the contract's %v", path, val, prop["type"])
			}
		case "enum":
			values, ok := val.([]any)
			if !ok || len(values) == 0 {
				return fmt.Errorf("%s: enum must list the supported values", path)
			}
			if allowed, ok := prop["enum"].([]any); ok {
				for _, v := range values {
					if !slices.ContainsFunc(allowed, func(a any) bool { return reflect.DeepEqual(a, v) }) {
						return fmt.Errorf("%s: enum value %v is not one of the contract's %v", path, v, allowed)
					}
				}
			}
			prop[key] = val
		case "properties", "required":
			if err := narrowObject(path+".", prop, map[string]any{key: val}); err != nil {
				return err
			}
		case "description":
			return fmt.Errorf("%s: description belongs to the contract", path)
		default:
			prop[key] = val
		}
	}
	return nil
}

func deepCopyMap(m map[string]any) map[string]any {
	data, _ := json.Marshal(m)
	var out map[string]any
	_ = json.Unmarshal(data, &out)
	return out
}
