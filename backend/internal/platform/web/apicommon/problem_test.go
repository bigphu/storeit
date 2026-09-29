package apicommon_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"testing"

	"storeit/internal/platform/errs"
	"storeit/internal/platform/web/apicommon"
)

// Body lỗi server thật sự gửi (errs.Error) phải khớp schema Problem trong
// api/common.yaml, vì frontend sinh type từ spec. Field lạ = spec lệch code.
func TestProblemSchemaMatchesErrs(t *testing.T) {
	e := errs.Unprocessable("/errors/validation-failed", "Validation failed",
		errs.WithDetail("The request body has 1 invalid field."),
		errs.WithFields(errs.FieldError{Field: "email", Detail: "is required"}))
	b, err := json.Marshal(e)
	if err != nil {
		t.Fatal(err)
	}

	dec := json.NewDecoder(bytes.NewReader(b))
	dec.DisallowUnknownFields()
	var p apicommon.Problem
	if err := dec.Decode(&p); err != nil {
		t.Fatalf("errs JSON %s does not fit apicommon.Problem: %v", b, err)
	}

	if p.Type != "/errors/validation-failed" || p.Title != "Validation failed" || p.Status != http.StatusUnprocessableEntity {
		t.Errorf("problem = %+v", p)
	}
	if p.Detail == nil || *p.Detail != "The request body has 1 invalid field." {
		t.Errorf("detail = %v", p.Detail)
	}
	if p.Errors == nil || len(*p.Errors) != 1 || (*p.Errors)[0] != (apicommon.FieldError{Field: "email", Detail: "is required"}) {
		t.Errorf("errors = %v", p.Errors)
	}
}
