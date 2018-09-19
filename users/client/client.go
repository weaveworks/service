package client

import (
	log "github.com/sirupsen/logrus"

	common_grpc "github.com/weaveworks/service/common/grpc"
	"github.com/weaveworks/service/users"
)

// New is a factory for Authenticators
func New(kind, address string, opts CachingClientConfig) (users.UsersClient, error) {
	var client users.UsersClient
	var err error
	switch kind {
	case "mock":
		client = MockClient{}
	case "grpc":
		client, err = newGRPCClient(address)
		if err != nil {
			return nil, err
		}
	default:
		log.Fatal("Incorrect authenticator type: ", kind)
		return nil, nil
	}
	if opts.CacheEnabled {
		client = newCachingClient(opts, client)
	}
	return client, nil
}

func newGRPCClient(address string) (users.UsersClient, error) {
	conn, err := common_grpc.NewInsecureConn(address, "", nil)
	if err != nil {
		return nil, err
	}
	return users.NewUsersClient(conn), nil
}
