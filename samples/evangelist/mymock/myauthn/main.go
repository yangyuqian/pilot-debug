package main

import (
	"flag"
	"log"
	"net/http"
	"net/http/httputil"
	"strings"
)

// myauthn \
//    --target mock1.evangelist.svc.cluster.local \
//    --url-prefix /a/b
var (
	addr       = flag.String("addr", ":8080", "address for this reverse proxy")
	targetAddr = flag.String("target-addr", "", "target address to do reverse proxy")
	urlPrefix  = flag.String("url-prefix", "", "url prefix to determine if this proxy will serve inbound traffic or not")
)

type userInfo struct {
	Name  string
	Group string
}

// mapping token -> userInfos
var users = map[string]userInfo{
	"token1": userInfo{"user1", "group1"},
	"token2": userInfo{"user2", "group2"},
}

const (
	userHeader  = "x-evangelist-myuser"
	groupHeader = "x-evangelist-mygroup"
)

func main() {
	flag.Parse()

	http.ListenAndServe(*addr, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ok, info := authn(r)
		if !ok {
			log.Printf("unauthorized ...")
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		if !strings.HasPrefix(r.URL.Path, *urlPrefix) {
			log.Printf("not found url prefix %s for %s", *urlPrefix, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
			return
		}

		log.Printf("authorized with user: %s, group: %s", info.Name, info.Group)

		r.Header.Set(userHeader, info.Name)
		r.Header.Set(groupHeader, info.Group)

		target := r.URL
		target.Host = *targetAddr
		target.Scheme = "http"

		hparts := strings.Split(target.Host, ":")
		r.Header.Set("Host", hparts[0])
		r.Host = hparts[0]
		httputil.NewSingleHostReverseProxy(target).ServeHTTP(w, r)
	}))
}

// verify the incoming token
func authn(r *http.Request) (ok bool, info userInfo) {
	token := r.Header.Get("Authorization")
	token = strings.Replace(token, "Bearer ", "", 1)
	log.Printf("authenticating with token %s", token)

	if info, ok = users[token]; ok {
		return ok, info
	}

	return
}
