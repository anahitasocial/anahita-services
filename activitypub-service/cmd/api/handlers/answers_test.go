package handlers

import (
	"os"
	"testing"

	"anahita-libs/apidoc"
)

// THE RULE THIS GUARDS: what the handlers answer in these tests is what
// docs/api/activitypub-service.json says they answer (docs/api.md, Describing a
// route). The helpers that call a handler keep each answer here.
var answers = apidoc.NewAnswers("../../../../docs/api/activitypub-service.json")

func TestMain(m *testing.M) { os.Exit(answers.Run(m)) }
