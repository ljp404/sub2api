package service

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"maps"
	"mime"
	"mime/multipart"
	"slices"
	"strings"
)

func rewriteImageSizeFromQuote(body []byte, contentType string, quote *ParameterPricingSnapshot) ([]byte, string, error) {
	fields := make(map[string]string, 2)
	for _, key := range []string{"resolution", "aspect_ratio"} {
		if value := quote.Input.Dimensions[key]; value != "" {
			fields[key] = value
		}
	}
	// Drop all client size aliases; only frozen, priced dimensions reach upstream.
	remove := map[string]bool{"size": true, "image_size": true, "resolution": true, "aspect_ratio": true}
	mediaType, params, err := mime.ParseMediaType(contentType)
	if err == nil && strings.EqualFold(mediaType, "multipart/form-data") {
		boundary := params["boundary"]
		if boundary == "" {
			return nil, "", fmt.Errorf("multipart boundary is required")
		}
		reader := multipart.NewReader(bytes.NewReader(body), boundary)
		var buffer bytes.Buffer
		writer := multipart.NewWriter(&buffer)
		for {
			part, err := reader.NextPart()
			if err == io.EOF {
				break
			}
			if err != nil {
				return nil, "", err
			}
			if remove[part.FormName()] {
				_ = part.Close()
				continue
			}
			target, err := writer.CreatePart(cloneMultipartHeader(part.Header))
			if err != nil {
				_ = part.Close()
				return nil, "", err
			}
			_, err = io.Copy(target, part)
			_ = part.Close()
			if err != nil {
				return nil, "", err
			}
		}
		for _, key := range slices.Sorted(maps.Keys(fields)) {
			if err := writer.WriteField(key, fields[key]); err != nil {
				return nil, "", err
			}
		}
		if err := writer.Close(); err != nil {
			return nil, "", err
		}
		return buffer.Bytes(), writer.FormDataContentType(), nil
	}
	var payload map[string]json.RawMessage
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, "", err
	}
	for key := range remove {
		delete(payload, key)
	}
	for key, value := range fields {
		payload[key], _ = json.Marshal(value)
	}
	rewritten, err := json.Marshal(payload)
	return rewritten, contentType, err
}
