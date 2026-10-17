package requests

import (
	"fmt"
	"reflect"
	"strings"
	"testing"

	"anahita-libs/constants"
)

// The lengths these requests take are the ones constants/limits.go names,
// which the web app holds to as well (src/constants/limits.js). A struct
// tag cannot name a constant, so this says when one has drifted.
func TestTheLengthsAreTheStandardOnes(t *testing.T) {
	for _, c := range []struct {
		req   any
		field string
		max   int
	}{
		{PersonEditRequest{}, "Name", constants.DisplayNameMax},
		{PersonEditRequest{}, "Body", constants.BioMax},
		{ActorEditRequest{}, "Name", constants.DisplayNameMax},
		{ActorEditRequest{}, "Body", constants.BioMax},
		{ActorAddRequest{}, "Name", constants.DisplayNameMax},
		{ActorAddRequest{}, "Body", constants.BioMax},
		{MediumEditRequest{}, "Name", constants.TitleMax},
		{MediumEditRequest{}, "Body", constants.PostBodyMax},
		{MediumAddRequest{}, "Name", constants.TitleMax},
		{MediumAddRequest{}, "Body", constants.PostBodyMax},
	} {
		kind := reflect.TypeOf(c.req)
		field, ok := kind.FieldByName(c.field)
		if !ok {
			t.Errorf("%s has no %s", kind.Name(), c.field)
			continue
		}
		want := fmt.Sprintf("max=%d", c.max)
		if tag := field.Tag.Get("validate"); !strings.Contains(tag, want) {
			t.Errorf("%s.%s is validated as %q, want %s", kind.Name(), c.field, tag, want)
		}
	}
}
