// Package websub subscribes a Pulse to the hub its feed names, so that the
// hub says the moment something is published (WebSub, W3C 2018).
//
// What a hub sends is taken only as a sign to read the feed now: the feed is
// always read from the site itself, as it is on every other read, so nothing
// a hub says is believed. The address it calls back carries a signature of
// the Pulse's id, so that only a hub that was given it can bring a read
// forward.
package websub

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// Lease is how long a subscription is asked for. A hub may give less; it is
// asked again before what it gave runs out.
const Lease = 10 * 24 * time.Hour

// Signature is what proves a call back is about this Pulse.
func Signature(nodeID int64, key string) string {
	mac := hmac.New(sha256.New, []byte(key))
	fmt.Fprintf(mac, "websub:%d", nodeID)
	return hex.EncodeToString(mac.Sum(nil))[:32]
}

// Signed says whether a signature is this Pulse's.
func Signed(nodeID int64, key, signature string) bool {
	return key != "" && hmac.Equal([]byte(Signature(nodeID, key)), []byte(signature))
}

// Callback is where a hub calls back for a Pulse, under the API.
func Callback(api func(string) string, nodeID int64, key string) string {
	return api("/pulses/_websub/" + strconv.FormatInt(nodeID, 10) + "/" + Signature(nodeID, key))
}

// Subscribe asks a hub to call back when the topic changes. The hub then
// checks, by asking the callback, that this was asked: see the verification
// route in cmd/api.
func Subscribe(ctx context.Context, client *http.Client, hub, topic, callback string) error {
	form := url.Values{
		"hub.mode":          {"subscribe"},
		"hub.topic":         {topic},
		"hub.callback":      {callback},
		"hub.lease_seconds": {strconv.Itoa(int(Lease.Seconds()))},
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, hub, strings.NewReader(form.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("User-Agent", "Anahita/1.0 (Pulse; +https://anahita.io)")

	res, err := client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	// 202 is what the standard says; some hubs answer 204 or 200.
	if res.StatusCode < 200 || res.StatusCode > 299 {
		return fmt.Errorf("the hub answered %d", res.StatusCode)
	}
	return nil
}
