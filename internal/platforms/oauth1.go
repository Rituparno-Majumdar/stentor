package platforms

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha1"
	"encoding/base64"
	"fmt"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"
)

// oauth1Sign builds the Authorization header for one OAuth 1.0a
// "User Context" request (used by Twitter/X's classic consumer-key +
// access-token credential shape). method/rawURL must have no query
// string -- pass query params separately if ever needed.
func oauth1Sign(method, rawURL, consumerKey, consumerSecret, token, tokenSecret string) (string, error) {
	nonceBytes := make([]byte, 16)
	if _, err := rand.Read(nonceBytes); err != nil {
		return "", err
	}
	nonce := base64.RawURLEncoding.EncodeToString(nonceBytes)
	timestamp := strconv.FormatInt(time.Now().Unix(), 10)

	params := map[string]string{
		"oauth_consumer_key":     consumerKey,
		"oauth_nonce":            nonce,
		"oauth_signature_method": "HMAC-SHA1",
		"oauth_timestamp":        timestamp,
		"oauth_token":            token,
		"oauth_version":          "1.0",
	}

	keys := make([]string, 0, len(params))
	for k := range params {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, pctEncode(k)+"="+pctEncode(params[k]))
	}
	paramString := strings.Join(parts, "&")

	base := strings.ToUpper(method) + "&" + pctEncode(rawURL) + "&" + pctEncode(paramString)
	signingKey := pctEncode(consumerSecret) + "&" + pctEncode(tokenSecret)

	mac := hmac.New(sha1.New, []byte(signingKey))
	mac.Write([]byte(base))
	signature := base64.StdEncoding.EncodeToString(mac.Sum(nil))
	params["oauth_signature"] = signature

	headerKeys := make([]string, 0, len(params))
	for k := range params {
		headerKeys = append(headerKeys, k)
	}
	sort.Strings(headerKeys)
	headerParts := make([]string, 0, len(headerKeys))
	for _, k := range headerKeys {
		headerParts = append(headerParts, fmt.Sprintf(`%s="%s"`, pctEncode(k), pctEncode(params[k])))
	}
	return "OAuth " + strings.Join(headerParts, ", "), nil
}

// pctEncode is RFC 3986 percent-encoding, which OAuth 1.0a requires and
// net/url.QueryEscape does not quite match (it encodes space as "+").
func pctEncode(s string) string {
	escaped := url.QueryEscape(s)
	escaped = strings.ReplaceAll(escaped, "+", "%20")
	return escaped
}
