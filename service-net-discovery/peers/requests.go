package peers

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/weaveworks/common/user"
)

const (
	httpPeerNameField  = "peername"
	httpNickNameField  = "nickname"
	httpAddressesField = "addresses"
)

func badRequest(w http.ResponseWriter, err error) {
	http.Error(w, err.Error(), http.StatusBadRequest)
	log.Warningln("BadRequest:", err.Error())
}

// PeerUpdateRequest represents an update to a peer that service-net will accept
type PeerUpdateRequest struct {
	Name      string   `json:"peername"`
	Nickname  string   `json:"nickname"`  // optional
	Addresses []string `json:"addresses"` // can be empty
}

//PeerUpdateResponse represents the response sent by service-net when a peer has been updated
type PeerUpdateResponse struct {
	Addresses []string `json:"addresses"`
	PeerCount int      `json:"peercount"`
}

// PeerInfoResponse represents peer information to be returned on a peer listing
type PeerInfoResponse struct {
	Name      string   `json:"peername"`
	Nickname  string   `json:"nickname"`  // optional
	Addresses []string `json:"addresses"` // can be empty
	LastSeen  string   `json:"lastseen"`  // format ?
}

/* Example call:
   curl -X POST -H X-Scope-OrgID:123 --data-binary \
   '{ "peername": "foo", "addresses": ["9.2.3.3","5.5.5.5" ] }' 127.0.0.1:8080/api/net/peer
*/

type params struct {
	PeerUpdateRequest
	ctx        context.Context
	instanceID string
}

func getParams(r *http.Request) (*params, error) {
	id, ctx, err := user.ExtractOrgIDFromHTTPRequest(r)
	if err != nil {
		return nil, err
	}
	var update PeerUpdateRequest
	err = json.NewDecoder(r.Body).Decode(&update)
	if err != nil {
		return nil, err
	}
	if update.Name == "" {
		return nil, fmt.Errorf("Must supply peername value")
	}
	return &params{PeerUpdateRequest: update, ctx: ctx, instanceID: id}, nil
}

// ListPeers is the endpoint for peer listing (/api/net/peer)
func (d *PeerDiscovery) ListPeers(w http.ResponseWriter, r *http.Request) {
	id, ctx, err := user.ExtractOrgIDFromHTTPRequest(r)
	if err != nil {
		badRequest(w, err)
		return
	}
	peers, err := d.peerList(ctx, id)
	if err != nil {
		badRequest(w, err)
		return
	}
	response := []PeerInfoResponse{}
	for _, peer := range peers {
		peerInfo := PeerInfoResponse{
			Name:     peer.PeerName,
			Nickname: peer.NickName,
			LastSeen: peer.LastSeen.Format(time.RFC3339), // format?
		}
		for _, addr := range peer.Addresses {
			peerInfo.Addresses = append(peerInfo.Addresses, addr.String())
		}
		response = append(response, peerInfo)
	}
	json.NewEncoder(w).Encode(response)
}

// UpdatePeer is the peer update endpoint for service-net
func (d *PeerDiscovery) UpdatePeer(w http.ResponseWriter, r *http.Request) {
	timestamp := time.Now().UTC()
	update, err := getParams(r)
	if err != nil {
		badRequest(w, err)
		return
	}
	addresses := []net.IP{}
	for _, a := range update.Addresses {
		addr := net.ParseIP(a)
		if addr == nil {
			badRequest(w, fmt.Errorf("Invalid address: %s", a))
			return
		}
		addresses = append(addresses, addr)
	}
	peers, peerCount, err := d.updatePeer(update.ctx, update.instanceID, update.Name, update.Nickname, timestamp, addresses)
	if err != nil {
		badRequest(w, err)
		return
	}
	log.Info("peer update ", update.instanceID, peers)
	response := PeerUpdateResponse{Addresses: peers, PeerCount: peerCount}
	json.NewEncoder(w).Encode(response)
}

// DeletePeer deletes a peer
func (d *PeerDiscovery) DeletePeer(w http.ResponseWriter, r *http.Request) {
	update, err := getParams(r)
	if err != nil {
		badRequest(w, err)
		return
	}
	err = d.deletePeer(update.ctx, update.instanceID, update.Name)
	if err != nil {
		badRequest(w, err)
		return
	}
	log.Infof("peer delete %s/%s", update.instanceID, update.Name)
}
