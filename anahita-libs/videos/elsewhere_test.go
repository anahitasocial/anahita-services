package videos

import (
	"strings"
	"testing"
)

// A post's file from elsewhere sits in its meta beside its card, a copy goes
// in and comes out without taking the rest, and an address that is not
// https is not kept (roadmap row 120).
func TestWhereAFileFromElsewhereIsKept(t *testing.T) {
	card := []byte(`{"link_preview":{"url":"https://a.example"}}`)
	e := &Elsewhere{Kind: ElsewhereVideo, Source: "https://pds.example/xrpc/x?cid=1", Page: "https://bsky.app/p/1", Site: "bsky.app"}

	meta := WithElsewhere(card, e)
	if got := ElsewhereFromMeta(meta); got == nil || got.Source != e.Source || got.Host() != "pds.example" {
		t.Fatalf("read back: %+v", got)
	}
	if !strings.Contains(string(meta), "link_preview") {
		t.Errorf("the card went: %s", meta)
	}

	withVideo, err := IntoMeta(meta, &Video{Renditions: []Rendition{{Height: 720, Key: "k"}}})
	if err != nil || FromMeta(withVideo) == nil {
		t.Fatalf("copy in: %v", err)
	}
	without := WithoutVideo(withVideo)
	if FromMeta(without) != nil || ElsewhereFromMeta(without) == nil || !strings.Contains(string(without), "link_preview") {
		t.Errorf("copy out: %s", without)
	}

	for _, bad := range []*Elsewhere{
		{Kind: ElsewhereVideo, Source: "http://plain.example/v.mp4"},
		{Kind: "picture", Source: "https://a.example/v.mp4"},
		{Kind: ElsewhereVideo, Source: "https://user:pw@a.example/v.mp4"},
		nil,
	} {
		if got := WithElsewhere(card, bad); string(got) != string(card) {
			t.Errorf("%+v was kept: %s", bad, got)
		}
	}
}
