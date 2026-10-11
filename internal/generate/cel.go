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
	"fmt"
	"slices"
	"strings"

	"github.com/google/cel-go/cel"
)

// CELIssue is an x-kubernetes-validations rule the UI cannot evaluate.
type CELIssue struct {
	Source string
	Path   string
	Rule   string
	Reason string
}

func (i CELIssue) String() string {
	return fmt.Sprintf("%s: %s: rule %q: %s", i.Source, i.Path, i.Rule, i.Reason)
}

// ValidateCELRules checks every x-kubernetes-validations rule in the resolved
// parameter schemas and in the supportedFields declarations against the
// standard CEL library alone. The server evaluates these rules with the
// Kubernetes CEL libraries, but the UI evaluates them as the user types and
// loads none of them, so a rule that calls quantity(), isURL() or format.*
// would pass on the server and break the form.
func ValidateCELRules(cfg *AssembledConfig, schemas map[string]any) ([]CELIssue, error) {
	env, err := cel.NewEnv(
		cel.Variable("self", cel.DynType),
		cel.Variable("oldSelf", cel.DynType),
	)
	if err != nil {
		return nil, fmt.Errorf("building the CEL environment: %w", err)
	}

	var issues []CELIssue
	for _, name := range sortedKeys(schemas) {
		schema, _ := schemas[name].(map[string]any)
		issues = append(issues, celIssues(env, "type "+name, "", schema)...)
	}
	for _, d := range allDeclarations(cfg) {
		source := fmt.Sprintf("topology %s: component %q supportedFields", d.topology, d.component)
		issues = append(issues, celIssues(env, source, "", d.schema)...)
	}
	return issues, nil
}

// celIssues walks a schema and compiles each rule and messageExpression.
func celIssues(env *cel.Env, source, path string, schema map[string]any) []CELIssue {
	if schema == nil {
		return nil
	}

	var issues []CELIssue
	validations, _ := schema["x-kubernetes-validations"].([]any)
	for _, raw := range validations {
		validation, _ := raw.(map[string]any)
		for _, key := range []string{"rule", "messageExpression"} {
			expr, _ := validation[key].(string)
			if expr == "" {
				continue
			}
			if _, iss := env.Compile(expr); iss.Err() != nil {
				issues = append(issues, CELIssue{
					Source: source,
					Path:   pathOrRoot(path),
					Rule:   expr,
					Reason: "uses more than the standard CEL library, which is all the UI evaluates: " +
						compileErrors(iss),
				})
			}
		}
	}

	properties, _ := schema["properties"].(map[string]any)
	for _, name := range sortedKeys(properties) {
		property, _ := properties[name].(map[string]any)
		issues = append(issues, celIssues(env, source, path+"."+name, property)...)
	}
	if items, ok := schema["items"].(map[string]any); ok {
		issues = append(issues, celIssues(env, source, path+"[*]", items)...)
	}
	if additional, ok := schema["additionalProperties"].(map[string]any); ok {
		issues = append(issues, celIssues(env, source, path+"[*]", additional)...)
	}
	for _, key := range []string{"allOf", "anyOf", "oneOf"} {
		branches, _ := schema[key].([]any)
		for _, raw := range branches {
			branch, _ := raw.(map[string]any)
			issues = append(issues, celIssues(env, source, path, branch)...)
		}
	}
	return issues
}

// compileErrors lists each distinct compiler message once, without the
// source excerpt cel-go appends to every error.
func compileErrors(iss *cel.Issues) string {
	var messages []string
	for _, err := range iss.Errors() {
		if !slices.Contains(messages, err.Message) {
			messages = append(messages, err.Message)
		}
	}
	return strings.Join(messages, "; ")
}

func pathOrRoot(path string) string {
	if path == "" {
		return "."
	}
	return path
}
