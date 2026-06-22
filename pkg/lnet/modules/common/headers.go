// common/headers.go — allowlista nagłówków forwardowanych do upstream.
//
// Nagłówki spoza allowlisty → ciche odrzucenie, nie forwardowane.
// Bezpieczeństwo: nie przepuszczamy niczego czego upstream nie oczekuje.

package common

import (
	"net/http"
	"net/textproto"
)

// filterHeaders zwraca tylko nagłówki z allowlisty (case-insensitive).
// Nagłówki spoza listy są cicho odrzucane — nie forwardowane.
func FilterHeaders(src http.Header) http.Header {
	dst := make(http.Header)
	for k, v := range src {
		if allowedHeader(k) {
			dst[k] = v
		}
	}
	return dst
}

var allowedHeaders = map[string]bool{
	"Authorization": true,
	"Content-Type":  true,
	"Accept":        true,
	"User-Agent":    true,
}

func allowedHeader(key string) bool {
	return allowedHeaders[textproto.CanonicalMIMEHeaderKey(key)]
}
