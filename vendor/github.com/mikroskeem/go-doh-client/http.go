package doh

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
)

// exchangeHTTPS sends a given query to a given resolver using a DoH POST
// request as described in RFC 8484, and returns the response's body.
// Returns an error if there was an issue sending the request or reading the
// response body.
func (r *Resolver) exchangeHTTPS(ctx context.Context, q []byte) (a []byte, err error) {
	url := fmt.Sprintf("https://%s/dns-query", r.Host)
	body := bytes.NewBuffer(q)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, body)
	if err != nil {
		return
	}

	req.Header.Add("Accept", "application/dns-message")
	req.Header.Add("Content-Type", "application/dns-message")

	client := r.HTTPClient
	if client == nil {
		client = http.DefaultClient
	}

	resp, err := client.Do(req)
	if errors.Is(err, context.DeadlineExceeded) {
		err = ErrTimeout
		return
	} else if err != nil {
		return
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		err = fmt.Errorf("HTTPS server returned with non-OK code %d", resp.StatusCode)
		return
	}

	return ioutil.ReadAll(resp.Body)
}
