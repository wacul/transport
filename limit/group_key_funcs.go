package limit

import "net/http"

// GroupKeyByHost makes key of the request the Host name
func GroupKeyByHost(r *http.Request) string {
	return r.URL.Host
}
