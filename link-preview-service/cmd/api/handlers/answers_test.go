package handlers

import (
	"os"
	"testing"

	"anahita-libs/apidoc"
)

// THE RULE THIS GUARDS: what the handlers answer in these tests is what
// docs/api/link-preview-service.json says they answer (docs/api.md, Describing a
// route). The helpers that call a handler keep each answer here.
var answers = apidoc.NewAnswers("../../../../docs/api/link-preview-service.json")

func TestMain(m *testing.M) { os.Exit(answers.Run(m)) }
