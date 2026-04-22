// Package slack — request signature verification.
//
// Slack signs every request to a slash-command URL with HMAC-SHA256 over
// "v0:<timestamp>:<raw-body>" using the app's signing secret. We must verify
// this before trusting any form field, otherwise anyone can forge a slash
// command POST and submit tasks. See:
// https://api.slack.com/authentication/verifying-requests-from-slack
package slack

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"math"
	"net/http"
	"strconv"
	"time"
)

const (
	slackSignatureVersion = "v0"
	// maxTimestampAge bounds replay attacks. Slack recommends 5 minutes.
	maxTimestampAge = 5 * time.Minute
	// maxSlackBodySize caps the request body to prevent denial-of-service.
	// Slack slash-command payloads are small (<4 KB); 1 MB is generous.
	maxSlackBodySize int64 = 1 << 20
)

// verifySlackRequest validates X-Slack-Signature and X-Slack-Request-Timestamp
// headers against the request body using the configured signing secret.
//
// On success it returns the raw body bytes (callers must parse form values
// from these bytes; r.Body has been consumed).
//
// On failure it writes an HTTP error response and returns nil.
//
// If signingSecret is empty, verification is skipped (dev mode). The caller
// is responsible for surfacing this in startup logs.
func verifySlackRequest(w http.ResponseWriter, r *http.Request, signingSecret string) []byte {
	// Dev mode: no secret configured, skip verification but still consume body
	// so callers get a consistent return shape.
	if signingSecret == "" {
		body, err := io.ReadAll(io.LimitReader(r.Body, maxSlackBodySize+1))
		if err != nil {
			http.Error(w, "failed to read body", http.StatusBadRequest)
			return nil
		}
		if int64(len(body)) > maxSlackBodySize {
			http.Error(w, "request body too large", http.StatusRequestEntityTooLarge)
			return nil
		}
		return body
	}

	ts := r.Header.Get("X-Slack-Request-Timestamp")
	sig := r.Header.Get("X-Slack-Signature")
	if ts == "" || sig == "" {
		http.Error(w, "missing Slack signature headers", http.StatusUnauthorized)
		return nil
	}

	// Reject stale or future-dated requests (replay protection).
	tsInt, err := strconv.ParseInt(ts, 10, 64)
	if err != nil {
		http.Error(w, "invalid timestamp", http.StatusBadRequest)
		return nil
	}
	age := time.Duration(math.Abs(float64(time.Now().Unix()-tsInt))) * time.Second
	if age > maxTimestampAge {
		http.Error(w, "request too old", http.StatusUnauthorized)
		return nil
	}

	body, err := io.ReadAll(io.LimitReader(r.Body, maxSlackBodySize+1))
	if err != nil {
		http.Error(w, "failed to read body", http.StatusBadRequest)
		return nil
	}
	if int64(len(body)) > maxSlackBodySize {
		http.Error(w, "request body too large", http.StatusRequestEntityTooLarge)
		return nil
	}

	// Compute v0=HMAC-SHA256(secret, "v0:<ts>:<body>") and compare.
	baseString := fmt.Sprintf("%s:%s:%s", slackSignatureVersion, ts, string(body))
	mac := hmac.New(sha256.New, []byte(signingSecret))
	mac.Write([]byte(baseString))
	expected := slackSignatureVersion + "=" + hex.EncodeToString(mac.Sum(nil))

	if !hmac.Equal([]byte(sig), []byte(expected)) {
		http.Error(w, "invalid signature", http.StatusUnauthorized)
		return nil
	}

	return body
}
