package peers

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"net"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/prometheus/client_golang/prometheus"
	log "github.com/sirupsen/logrus"
	"github.com/weaveworks/common/instrument"
	"golang.org/x/net/context"
)

const (
	instanceField = "i"
	peerNameField = "p"
	peerDataField = "d"

	// MetricsNamespace for the service
	MetricsNamespace = "service_net"
)

var (
	dynamoRequestDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: MetricsNamespace,
		Name:      "dynamo_request_duration_seconds",
		Help:      "Time in seconds spent doing DynamoDB requests.",
		Buckets:   prometheus.DefBuckets,
	}, []string{"method", "status_code"})
	dynamoConsumedCapacity = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: MetricsNamespace,
		Name:      "dynamo_consumed_capacity_total",
		Help:      "The capacity units consumed by operation.",
	}, []string{"operation"})
)

func init() {
	prometheus.MustRegister(dynamoRequestDuration)
	prometheus.MustRegister(dynamoConsumedCapacity)
}

// PeerInfo represents a peer in a serializable way
type PeerInfo struct {
	PeerName  string
	NickName  string
	Addresses []net.IP
	LastSeen  time.Time
}

// PeerDiscovery refers to the peer info in the DynamoDb instance
type PeerDiscovery struct {
	tableName string
	db        *dynamodb.DynamoDB
}

// PeerDiscoveryConfig has everything we need to make a PeerDiscovery
type PeerDiscoveryConfig struct {
	DynamodbURL string
}

// New will create a new PeerDiscovery
func New(config PeerDiscoveryConfig) (*PeerDiscovery, error) {
	dynamoDBConfig, tableName, err := awsConfigFromURLString(config.DynamodbURL)
	if err != nil {
		return nil, err
	}
	d := &PeerDiscovery{
		tableName: tableName,
		db:        dynamodb.New(session.New(dynamoDBConfig)),
	}
	err = d.CreateTables()
	if err != nil {
		return nil, err
	}
	return d, nil
}

// CreateTables will create the dynamoDB tables for peers if they do not already exist
func (d *PeerDiscovery) CreateTables() error {
	// see if tableName exists
	tableFound := false
	lpi := dynamodb.ListTablesInput{Limit: aws.Int64(50)}
	err := d.db.ListTablesPages(&lpi, func(resp *dynamodb.ListTablesOutput, lastPage bool) bool {
		for _, s := range resp.TableNames {
			if *s == d.tableName {
				tableFound = true
				return false
			}
		}
		return true
	})
	if err != nil {
		return err
	}
	if tableFound {
		return nil
	}

	params := &dynamodb.CreateTableInput{
		TableName: aws.String(d.tableName),
		AttributeDefinitions: []*dynamodb.AttributeDefinition{
			{
				AttributeName: aws.String(instanceField),
				AttributeType: aws.String("S"),
			},
			{
				AttributeName: aws.String(peerNameField),
				AttributeType: aws.String("S"),
			},
		},
		KeySchema: []*dynamodb.KeySchemaElement{
			{
				AttributeName: aws.String(instanceField),
				KeyType:       aws.String("HASH"),
			},
			{
				AttributeName: aws.String(peerNameField),
				KeyType:       aws.String("RANGE"),
			},
		},
		ProvisionedThroughput: &dynamodb.ProvisionedThroughput{
			ReadCapacityUnits:  aws.Int64(1), // Table should be pre-created in dev and prod,
			WriteCapacityUnits: aws.Int64(1), // so these values are never used in anger
		},
	}
	log.Infof("Creating table %s", d.tableName)
	_, err = d.db.CreateTable(params)
	return err
}

func (d *PeerDiscovery) updatePeer(ctx context.Context, instanceID, fromPeerName, nickname string, timestamp time.Time, addresses []net.IP) (peerAddresses []string, peerCount int, err error) {
	fromPeer := PeerInfo{
		PeerName:  fromPeerName,
		NickName:  nickname,
		Addresses: addresses,
		LastSeen:  timestamp, // TODO: should we code for the possibility time went backwards?
	}

	buf := new(bytes.Buffer)
	enc := gob.NewEncoder(buf)
	if err := enc.Encode(fromPeer); err != nil {
		return nil, 0, err
	}

	var resp *dynamodb.PutItemOutput
	err = instrument.TimeRequestHistogram(ctx, "DynamoDB.PutItem", dynamoRequestDuration, func(_ context.Context) error {
		resp, err = d.db.PutItem(&dynamodb.PutItemInput{
			TableName: aws.String(d.tableName),
			Item: map[string]*dynamodb.AttributeValue{
				instanceField: {S: aws.String(instanceID)},
				peerNameField: {S: aws.String(fromPeerName)},
				peerDataField: {B: buf.Bytes()},
			},
			ReturnConsumedCapacity: aws.String(dynamodb.ReturnConsumedCapacityTotal),
		})
		return err
	})
	if err != nil {
		return nil, 0, err
	}
	if resp.ConsumedCapacity != nil && resp.ConsumedCapacity.CapacityUnits != nil {
		dynamoConsumedCapacity.WithLabelValues("DynamoDB.PutItem").Add(*resp.ConsumedCapacity.CapacityUnits)
	}

	peers, err := d.peerList(ctx, instanceID)
	if err != nil {
		return nil, 0, err
	}
	peerCount = len(peers)               // note count includes the calling peer
	addrMap := make(map[string]struct{}) // map to de-dupe
	for _, peer := range peers {
		if peer.PeerName != fromPeerName { // remove the requesting peer's own addresses
			for _, addr := range peer.Addresses {
				addrMap[addr.String()] = struct{}{}
			}
		}
	}
	for addr := range addrMap {
		peerAddresses = append(peerAddresses, addr)
	}
	return
}

func (d *PeerDiscovery) peerList(ctx context.Context, instanceID string) (peers []PeerInfo, err error) {
	var resp *dynamodb.QueryOutput
	err = instrument.TimeRequestHistogram(ctx, "DynamoDB.Query", dynamoRequestDuration, func(_ context.Context) error {
		var err error
		resp, err = d.db.Query(&dynamodb.QueryInput{
			TableName:              aws.String(d.tableName),
			ReturnConsumedCapacity: aws.String(dynamodb.ReturnConsumedCapacityTotal),
			KeyConditionExpression: aws.String(instanceField + " = :id"),
			ExpressionAttributeValues: map[string]*dynamodb.AttributeValue{
				":id": {S: aws.String(instanceID)},
			}})
		return err
	})
	if err != nil {
		return nil, err
	}
	if resp.ConsumedCapacity != nil && resp.ConsumedCapacity.CapacityUnits != nil {
		dynamoConsumedCapacity.WithLabelValues("DynamoDB.Query").Add(*resp.ConsumedCapacity.CapacityUnits)
	}

	log.Debugf("Query returned: %v", resp)

	for i, item := range resp.Items {
		peerData, found := item[peerDataField]
		if !found {
			log.Errorf("Row %d has no data", i)
			continue
		}
		var peer PeerInfo
		reader := bytes.NewReader(peerData.B)
		decoder := gob.NewDecoder(reader)
		err := decoder.Decode(&peer)
		if err != nil {
			return nil, fmt.Errorf("error while decoding peer data: %s", err)
		}
		peers = append(peers, peer)
	}
	log.Debugf("peerList [%s] returning %v", instanceID, peers)
	return
}

func (d *PeerDiscovery) deletePeer(ctx context.Context, instanceID, peerName string) (err error) {
	var resp *dynamodb.DeleteItemOutput
	err = instrument.TimeRequestHistogram(ctx, "DynamoDB.DeleteItem", dynamoRequestDuration, func(_ context.Context) error {
		resp, err = d.db.DeleteItem(&dynamodb.DeleteItemInput{
			TableName: aws.String(d.tableName),
			Key: map[string]*dynamodb.AttributeValue{
				instanceField: {S: aws.String(instanceID)},
				peerNameField: {S: aws.String(peerName)},
			},
			ReturnConsumedCapacity: aws.String(dynamodb.ReturnConsumedCapacityTotal),
		})
		return err
	})
	if err != nil {
		return err
	}
	if resp.ConsumedCapacity != nil && resp.ConsumedCapacity.CapacityUnits != nil {
		dynamoConsumedCapacity.WithLabelValues("DynamoDB.DeleteItem").Add(*resp.ConsumedCapacity.CapacityUnits)
	}
	return nil
}
