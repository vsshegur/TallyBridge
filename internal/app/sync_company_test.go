package app

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestFilterSyncCompaniesHonorsExclusionsCaseInsensitively(t *testing.T) {
	cfg := Settings{SyncExcludedCompanies: []string{"Vipul Textiles"}}
	got := filterSyncCompanies([]Company{{Name: "Vijayalaxmi Textiles"}, {Name: "VIPUL TEXTILES"}}, cfg)
	if len(got) != 1 || got[0].Name != "Vijayalaxmi Textiles" {
		t.Fatalf("unexpected: %#v", got)
	}
}
func TestCompanySyncEndpointPersistsRemoveAndRestore(t *testing.T) {
	s, e := NewServer(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	call := func(enabled bool) {
		v := "false"
		if enabled {
			v = "true"
		}
		req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/company-sync", strings.NewReader(`{"company":"Vipul Textiles","enabled":`+v+`}`))
		rr := httptest.NewRecorder()
		s.handleCompanySync(rr, req)
		if rr.Code != http.StatusOK {
			t.Fatalf("%d %s", rr.Code, rr.Body.String())
		}
	}
	call(false)
	if companySyncEnabled(s.store.Settings(), "vipul textiles") {
		t.Fatal("remove failed")
	}
	call(true)
	if !companySyncEnabled(s.store.Settings(), "vipul textiles") {
		t.Fatal("restore failed")
	}
}
