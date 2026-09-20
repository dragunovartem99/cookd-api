package chat

import (
	"fmt"
	"testing"

	"github.com/dragunovartem99/cookd-api/internal/store"
)

// conversation alternates user and assistant, ending on a user message.
func conversation(n int) []store.Message {
	var h []store.Message
	for i := range n {
		role := "user"
		if i%2 == 1 {
			role = "assistant"
		}
		h = append(h, store.Message{Role: role, Text: fmt.Sprint(i)})
	}
	return h
}

func TestRecentKeepsShortConversationsWhole(t *testing.T) {
	if got := recent(conversation(11)); len(got) != 11 {
		t.Errorf("kept %d of 11", len(got))
	}
}

func TestRecentTrimsInJumps(t *testing.T) {
	seen := map[string]bool{}
	for n := 1; n <= 41; n += 2 {
		got := recent(conversation(n))
		if got[0].Role != "user" || got[len(got)-1].Text != fmt.Sprint(n-1) {
			t.Fatalf("n=%d: window %s..%s", n, got[0].Text, got[len(got)-1].Text)
		}
		if len(got) < min(n, keepMessages) {
			t.Errorf("n=%d: kept only %d", n, len(got))
		}
		seen[got[0].Text] = true
	}
	// 21 turns, but the window's start moved only a few times: the cached
	// prefix is stable in between.
	if len(seen) > 5 {
		t.Errorf("window start moved %d times", len(seen))
	}
}

func TestToParamsDropsOldPhotos(t *testing.T) {
	img := []store.Image{{MediaType: "image/jpeg", Data: []byte{1}}}
	h := conversation(7)
	h[0].Images, h[4].Images, h[6].Images = img, img, img

	params := toParams(h, Memory{})
	photos := func(n int) (images, notes int) {
		for _, b := range params[n].Content {
			if b.OfImage != nil {
				images++
			}
			if b.OfText != nil && b.OfText.Text == "[1 photo(s) sent here, no longer shown]" {
				notes++
			}
		}
		return
	}
	if i, n := photos(0); i != 0 || n != 1 {
		t.Errorf("old photo: %d images, %d notes", i, n)
	}
	if i, n := photos(4); i != 1 || n != 0 {
		t.Errorf("recent photo: %d images, %d notes", i, n)
	}
	if i, _ := photos(6); i != 1 {
		t.Errorf("latest photo missing")
	}
}
