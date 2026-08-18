package generate

import (
	"encoding/json"
	"testing"
)

func TestParseKubebuilderDirectives(t *testing.T) {
	doc := `
Some description
+kubebuilder:validation:Minimum=0
+kubebuilder:validation:Maximum=22
+kubebuilder:default=60
`
	schema := &openAPISchema{}
	parseKubebuilderDirectives(doc, schema)

	if schema.Minimum == nil || *schema.Minimum != 0 {
		t.Errorf("expected minimum 0, got %v", schema.Minimum)
	}
	if schema.Maximum == nil || *schema.Maximum != 22 {
		t.Errorf("expected maximum 22, got %v", schema.Maximum)
	}
	if schema.Default != float64(60) {
		t.Errorf("expected default 60, got %v", schema.Default)
	}
}

func TestResolveSchemasWithDirectives(t *testing.T) {
	schemas, err := ResolveSchemas([]string{"./testdata/testpkg"}, []string{"PxcPITRConfig"})
	if err != nil {
		t.Fatal(err)
	}

	schemaMap, ok := schemas["PxcPITRConfig"].(map[string]any)
	if !ok {
		t.Fatalf("expected map[string]any, got %T", schemas["PxcPITRConfig"])
	}

	b, _ := json.Marshal(schemaMap)
	var parsed openAPISchema
	json.Unmarshal(b, &parsed)

	props := parsed.Properties
	if props == nil {
		t.Fatal("expected properties")
	}

	tbu := props["timeBetweenUploads"]
	if tbu == nil || tbu.Minimum == nil || *tbu.Minimum != 1 || tbu.Default != float64(60) {
		t.Errorf("unexpected timeBetweenUploads schema: %+v", tbu)
	}

	ts := props["timeoutSeconds"]
	if ts == nil || ts.Maximum == nil || *ts.Maximum != 7200 || ts.Default != float64(3600) {
		t.Errorf("unexpected timeoutSeconds schema: %+v", ts)
	}
}
