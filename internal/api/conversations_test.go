package api

import (
	"encoding/json"
	"net/http"
	"testing"
)

func TestConversationsArePrivate(t *testing.T) {
	h := newHarness(t)
	id := h.newConversation(t, me)
	h.do("POST", "/conversations/"+id+"/messages", me, map[string]string{"text": "secret recipe"})

	for _, req := range [][2]string{
		{"GET", "/conversations/" + id},
		{"GET", "/conversations/" + id + "/messages"},
		{"DELETE", "/conversations/" + id},
		{"POST", "/conversations/" + id + "/messages"},
	} {
		if rec := h.do(req[0], req[1], other, map[string]string{"text": "hi"}); rec.Code != http.StatusNotFound {
			t.Errorf("%s %s as another user = %d, want 404", req[0], req[1], rec.Code)
		}
	}

	var list []conversationJSON
	json.Unmarshal(h.do("GET", "/conversations", other, nil).Body.Bytes(), &list)
	if len(list) != 0 {
		t.Errorf("another user lists %d conversations, want 0", len(list))
	}
}

func TestDeleteConversation(t *testing.T) {
	h := newHarness(t)
	id := h.newConversation(t, me)
	if rec := h.do("DELETE", "/conversations/"+id, me, nil); rec.Code != http.StatusNoContent {
		t.Fatalf("delete = %d", rec.Code)
	}
	if rec := h.do("GET", "/conversations/"+id, me, nil); rec.Code != http.StatusNotFound {
		t.Errorf("get after delete = %d, want 404", rec.Code)
	}
}
