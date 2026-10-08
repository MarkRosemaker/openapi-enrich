package enrich

import "testing"

func TestIsCustomHeader(t *testing.T) {
	custom := []string{"x-api-key", "x-request-id", "api-version", "api-key"}
	for _, h := range custom {
		if !isCustomHeader(h) {
			t.Errorf("expected %q to be custom header", h)
		}
	}

	notCustom := []string{"Accept", "Host", "Connection", "Cache-Control", "Accept-Language"}
	for _, h := range notCustom {
		if isCustomHeader(h) {
			t.Errorf("expected %q NOT to be custom header", h)
		}
	}
}
