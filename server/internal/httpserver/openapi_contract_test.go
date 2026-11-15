package httpserver

import (
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
)

// RT-34: the contract must document every status a handler can return deliberately.
func TestOpenAPIDocumentsApprovalFinalConflict(t *testing.T) {
	doc, err := openapi3.NewLoader().LoadFromFile("../../../api/openapi.yaml")
	if err != nil {
		t.Fatal(err)
	}
	op := doc.Paths.Find("/approvals/{approvalId}/decision").Put
	if op.Responses.Value("409") == nil {
		t.Fatal("PUT /approvals/{approvalId}/decision must document 409 approval_final (APR-03)")
	}
}
