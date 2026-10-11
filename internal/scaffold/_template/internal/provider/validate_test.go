package provider

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/openeverest/openeverest/v2/provider-runtime/controller/validationtest"
)

// TestValidate runs the checks the runtime runs before every Sync: the
// schemas, supportedFields and catalog this provider publishes, then Validate.
func TestValidate(t *testing.T) {
	t.Parallel()
	v := validationtest.New(t, New(), "../../charts/[[ .ProjectName ]]/generated/provider-spec.yaml")

	tests := []struct {
		fixture string
		// want lists the field paths of the expected errors.
		want []string
	}{
		{fixture: "testdata/validate/minimal.yaml"},
		// TODO: Add a fixture per rule, e.g.:
		// {
		//     fixture: "testdata/validate/even-replicas.yaml",
		//     want:    []string{"spec.components[engine].replicas"},
		// },
	}

	for _, tt := range tests {
		t.Run(tt.fixture, func(t *testing.T) {
			t.Parallel()
			res := v.Run(validationtest.Load(t, tt.fixture), nil)

			var got []string
			for _, err := range res.Errors {
				got = append(got, err.Field)
			}
			assert.Equal(t, tt.want, got, "errors: %v", res.Errors)
		})
	}
}
