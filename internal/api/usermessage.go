package api

import (
	"encoding/base64"
	"net/http"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/dragunovartem99/cookd-api/internal/store"
)

// imageTypes are the formats the model accepts.
var imageTypes = map[string]bool{"image/jpeg": true, "image/png": true, "image/gif": true, "image/webp": true}

// buildUserMessage validates a request body and turns it into a message.
// A photo's declared type must match what its bytes actually are.
func buildUserMessage(body sendRequest) (store.Message, error) {
	text := strings.TrimSpace(body.Text)
	if utf8.RuneCountInString(text) > maxText {
		return store.Message{}, badRequest("text is too long")
	}
	if text == "" && len(body.Images) == 0 {
		return store.Message{}, badRequest("text or an image is required")
	}
	if len(body.Images) > maxImages {
		return store.Message{}, badRequest("at most " + strconv.Itoa(maxImages) + " images per message")
	}

	msg := store.Message{Role: "user", Text: text}
	for _, in := range body.Images {
		if !imageTypes[in.MediaType] {
			return store.Message{}, badRequest("unsupported image type; use jpeg, png, gif or webp")
		}
		data, err := base64.StdEncoding.DecodeString(in.Data)
		if err != nil {
			return store.Message{}, badRequest("image data is not valid base64")
		}
		if len(data) == 0 || len(data) > maxImageBytes {
			return store.Message{}, badRequest("each image must be between 1 byte and 5 MiB")
		}
		if http.DetectContentType(data) != in.MediaType {
			return store.Message{}, badRequest("image bytes do not match their mediaType")
		}
		msg.Images = append(msg.Images, store.Image{MediaType: in.MediaType, Data: data})
	}
	return msg, nil
}
