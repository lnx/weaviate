//                           _       _
// __      _____  __ ___   ___  __ _| |_ ___
// \ \ /\ / / _ \/ _` \ \ / / |/ _` | __/ _ \
//  \ V  V /  __/ (_| |\ V /| | (_| | ||  __/
//   \_/\_/ \___|\__,_| \_/ |_|\__,_|\__\___|
//
//  Copyright © 2016 - 2021 SeMI Technologies B.V. All rights reserved.
//
//  CONTACT: hello@semi.technology
//

package db

import (
	"crypto/rand"
	"encoding/binary"

	"github.com/pkg/errors"
	"github.com/semi-technologies/weaviate/adapters/repos/db/helpers"
	"github.com/semi-technologies/weaviate/adapters/repos/db/inverted"
	"github.com/semi-technologies/weaviate/adapters/repos/db/lsmkv"
)

func (s *Shard) extendInvertedIndicesLSM(props []inverted.Property,
	docID uint64) error {
	for _, prop := range props {
		b := s.store.Bucket(helpers.BucketFromPropNameLSM(prop.Name))
		if b == nil {
			return errors.Errorf("no bucket for prop '%s' found", prop.Name)
		}

		hashBucket := s.store.Bucket(helpers.HashBucketFromPropNameLSM(prop.Name))
		if b == nil {
			return errors.Errorf("no hash bucket for prop '%s' found", prop.Name)
		}

		if prop.HasFrequency {
			for _, item := range prop.Items {
				if err := s.extendInvertedIndexItemWithFrequencyLSM(b, hashBucket, item,
					docID, item.TermFrequency); err != nil {
					return errors.Wrapf(err, "extend index with item '%s'",
						string(item.Data))
				}
			}
		} else {
			for _, item := range prop.Items {
				if err := s.extendInvertedIndexItemLSM(b, hashBucket, item, docID); err != nil {
					return errors.Wrapf(err, "extend index with item '%s'",
						string(item.Data))
				}
			}
		}
	}

	return nil
}

func (s *Shard) extendInvertedIndexItemWithFrequencyLSM(b, hashBucket *lsmkv.Bucket,
	item inverted.Countable, docID uint64, frequency float64) error {
	if b.Strategy() != lsmkv.StrategyMapCollection {
		panic("prop has frequency, but bucket does not have 'Map' strategy")
	}

	hash, err := generateRowHash()
	if err != nil {
		return err
	}

	if err := hashBucket.Put(item.Data, hash); err != nil {
		return err
	}

	buf := make([]byte, 16) // 8 bytes for doc id, 8 bytes for frequency
	binary.LittleEndian.PutUint64(buf[:8], docID)
	binary.LittleEndian.PutUint64(buf[8:], uint64(item.TermFrequency))

	pair := lsmkv.MapPair{
		Key:   buf[:8],
		Value: buf[8:],
	}

	return b.MapSet(item.Data, pair)
}

func (s *Shard) extendInvertedIndexItemLSM(b, hashBucket *lsmkv.Bucket,
	item inverted.Countable, docID uint64) error {
	if b.Strategy() != lsmkv.StrategySetCollection {
		panic("prop has no frequency, but bucket does not have 'Set' strategy")
	}

	hash, err := generateRowHash()
	if err != nil {
		return err
	}

	if err := hashBucket.Put(item.Data, hash); err != nil {
		return err
	}

	docIDBytes := make([]byte, 8)
	binary.LittleEndian.PutUint64(docIDBytes, docID)

	return b.SetAdd(item.Data, [][]byte{docIDBytes})
}

func (s *Shard) batchExtendInvertedIndexItemsLSMNoFrequency(b, hashBucket *lsmkv.Bucket,
	item inverted.MergeItem) error {
	if b.Strategy() != lsmkv.StrategySetCollection {
		panic("prop has no frequency, but bucket does not have 'Set' strategy")
	}

	hash, err := generateRowHash()
	if err != nil {
		return err
	}

	if err := hashBucket.Put(item.Data, hash); err != nil {
		return err
	}

	docIDs := make([][]byte, len(item.DocIDs))
	for i, idTuple := range item.DocIDs {
		docIDs[i] = make([]byte, 8)
		binary.LittleEndian.PutUint64(docIDs[i], idTuple.DocID)
	}

	return b.SetAdd(item.Data, docIDs)
}

// the row hash isn't actually a hash at this point, it is just a random
// sequence of bytes. The important thing is that every new write into this row
// replaces the hash as the read cacher will make a decision based on the hash
// if it should read the row again from cache. So changing the "hash" (by
// replacing it with other random bytes) is essentially just a signal to the
// read-time cacher to invalidate its entry
func generateRowHash() ([]byte, error) {
	out := make([]byte, 8)
	_, err := rand.Read(out)
	return out, err
}
