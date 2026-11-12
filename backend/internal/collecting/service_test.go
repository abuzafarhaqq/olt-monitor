package collecting_test

import (
	"strings"
	"testing"

	"github.com/abuzafarhaqq/olt-monitor/backend/internal/collecting"
)

// The bug this test exists for: a VSOL GPON OLT was handed the EPON adapter
// because lookup ignored technology.
func TestAdapterLookupRequiresTechnology(t *testing.T) {
	if _, _, err := collecting.AdapterFor("VSOL", "EPON"); err != nil {
		t.Fatalf("VSOL EPON should resolve: %v", err)
	}

	for _, pair := range [][2]string{{"VSOL", "GPON"}, {"C-Data", "GPON"}, {"VSOL", ""}} {
		_, _, err := collecting.AdapterFor(pair[0], pair[1])
		if err == nil {
			t.Fatalf("%s/%s resolved an adapter it does not have", pair[0], pair[1])
		}
		if !strings.Contains(err.Error(), "no adapter") {
			t.Fatalf("error should name the gap: %v", err)
		}
	}
}
