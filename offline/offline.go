// Package offline implements Dms3FsRouting with a client which
// is only able to perform offline operations.
package offline

import (
	"bytes"
	"context"
	"errors"
	"time"

	proto "github.com/gogo/protobuf/proto"
	cid "github.com/dms3-fs/go-cid"
	ds "github.com/dms3-fs/go-datastore"
	dshelp "github.com/dms3-fs/go-fs-ds-help"
	"github.com/dms3-p2p/go-p2p-peer"
	pstore "github.com/dms3-p2p/go-p2p-peerstore"
	record "github.com/dms3-p2p/go-p2p-record"
	pb "github.com/dms3-p2p/go-p2p-record/pb"
	routing "github.com/dms3-p2p/go-p2p-routing"
	ropts "github.com/dms3-p2p/go-p2p-routing/options"
)

// ErrOffline is returned when trying to perform operations that
// require connectivity.
var ErrOffline = errors.New("routing system in offline mode")

// NewOfflineRouter returns an Dms3FsRouting implementation which only performs
// offline operations. It allows to Put and Get signed dht
// records to and from the local datastore.
func NewOfflineRouter(dstore ds.Datastore, validator record.Validator) routing.Dms3FsRouting {
	return &offlineRouting{
		datastore: dstore,
		validator: validator,
	}
}

// offlineRouting implements the Dms3FsRouting interface,
// but only provides the capability to Put and Get signed dht
// records to and from the local datastore.
type offlineRouting struct {
	datastore ds.Datastore
	validator record.Validator
}

func (c *offlineRouting) PutValue(ctx context.Context, key string, val []byte, _ ...ropts.Option) error {
	if err := c.validator.Validate(key, val); err != nil {
		return err
	}
	if old, err := c.GetValue(ctx, key); err == nil {
		// be idempotent to be nice.
		if bytes.Equal(old, val) {
			return nil
		}
		// check to see if the older record is better
		i, err := c.validator.Select(key, [][]byte{val, old})
		if err != nil {
			// this shouldn't happen for validated records.
			return err
		}
		if i != 0 {
			return errors.New("can't replace a newer record with an older one")
		}
	}
	rec := record.MakePutRecord(key, val)
	data, err := proto.Marshal(rec)
	if err != nil {
		return err
	}

	return c.datastore.Put(dshelp.NewKeyFromBinary([]byte(key)), data)
}

func (c *offlineRouting) GetValue(ctx context.Context, key string, _ ...ropts.Option) ([]byte, error) {
	buf, err := c.datastore.Get(dshelp.NewKeyFromBinary([]byte(key)))
	if err != nil {
		return nil, err
	}

	rec := new(pb.Record)
	err = proto.Unmarshal(buf, rec)
	if err != nil {
		return nil, err
	}
	val := rec.GetValue()

	err = c.validator.Validate(key, val)
	if err != nil {
		return nil, err
	}
	return val, nil
}

func (c *offlineRouting) FindPeer(ctx context.Context, pid peer.ID) (pstore.PeerInfo, error) {
	return pstore.PeerInfo{}, ErrOffline
}

func (c *offlineRouting) FindProvidersAsync(ctx context.Context, k *cid.Cid, max int) <-chan pstore.PeerInfo {
	out := make(chan pstore.PeerInfo)
	close(out)
	return out
}

func (c *offlineRouting) Provide(_ context.Context, k *cid.Cid, _ bool) error {
	return ErrOffline
}

func (c *offlineRouting) Ping(ctx context.Context, p peer.ID) (time.Duration, error) {
	return 0, ErrOffline
}

func (c *offlineRouting) Bootstrap(context.Context) error {
	return nil
}

// ensure offlineRouting matches the Dms3FsRouting interface
var _ routing.Dms3FsRouting = &offlineRouting{}
