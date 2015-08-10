package main

import (
	"fmt"
	"net/http"
	"net"

	socks5 "github.com/armon/go-socks5"
)

func main() {
	go socksProxy()

	http.HandleFunc("/proxy.pac", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, `
function FindProxyForURL(url, host) {
	if (host == "run.weave.works" || shExpMatch(host, "*.weave.local")) {
			return "SOCKS5 localhost:8000";
	}

	return "DIRECT";
}
`)
	})
	if err := http.ListenAndServe(":8080", nil); err != nil {
		panic(err)
	}
}

type aliasingResolver struct {
	aliases map[string]string
	socks5.NameResolver
}

func (r aliasingResolver) Resolve(name string) (net.IP, error) {
	if alias, ok := r.aliases[name]; ok {
		return r.NameResolver.Resolve(alias)
	}
	return r.NameResolver.Resolve(name)
}

func socksProxy() {
	conf := &socks5.Config{
		Resolver: aliasingResolver{
			aliases: map[string]string{
				"run.weave.works": "frontend.weave.local",
			},
			NameResolver: socks5.DNSResolver{},
		},
	}
	server, err := socks5.New(conf)
	if err != nil {
		panic(err)
	}

	if err := server.ListenAndServe("tcp", ":8000"); err != nil {
		panic(err)
	}
}
