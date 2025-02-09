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

package hnsw

import (
	"bufio"
	"encoding/binary"
	"io"
	"os"
	"sort"

	"github.com/pkg/errors"
)

type mmapIndex struct {
	nodes               []mmapIndexNode
	connectionsPerLevel int
}

func (i *mmapIndex) UpsertNodeMaxLevel(node uint64, level uint16) {
	n := sort.Search(len(i.nodes), func(a int) bool {
		return i.nodes[a].id >= node
	})

	if n < len(i.nodes) && i.nodes[n].id == node {
		// update
		if i.nodes[n].maxLevel < level {
			i.nodes[n].maxLevel = level
		}
	} else {
		// insert

		// See https://github.com/golang/go/wiki/SliceTricks#insert
		i.nodes = append(i.nodes, mmapIndexNode{})
		copy(i.nodes[n+1:], i.nodes[n:])
		i.nodes[n].id = node
		i.nodes[n].maxLevel = level
	}
}

func (i *mmapIndex) DeleteNode(node uint64) {
}

type mmapIndexNode struct {
	id       uint64
	offset   uint64
	maxLevel uint16
}

func (n mmapIndexNode) Size(connectionsPerLevel int) int {
	return int(n.maxLevel)*2 + // overhead for uint16 length indicators
		connectionsPerLevel*int(n.maxLevel+1) // level 0 has 2x connections
}

type MmapCondensorAnalyzer struct {
	reader              *bufio.Reader
	connectionsPerLevel int
	index               mmapIndex
}

func newMmapCondensorAnalyzer(connectionsPerLevel int) *MmapCondensorAnalyzer {
	return &MmapCondensorAnalyzer{connectionsPerLevel: connectionsPerLevel}
}

func (a *MmapCondensorAnalyzer) Do(file *os.File) (mmapIndex, error) {
	a.reader = bufio.NewReaderSize(file, 1024*1024)

	a.index = mmapIndex{
		connectionsPerLevel: a.connectionsPerLevel,
		nodes:               make([]mmapIndexNode, 0, 10000),
	}

	if err := a.loop(); err != nil {
		return a.index, err
	}

	return a.index, nil
}

func (a *MmapCondensorAnalyzer) loop() error {
	for {
		ct, err := a.ReadCommitType(a.reader)
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}

			return err
		}

		switch ct {
		case AddNode:
			err = a.ReadNode(a.reader)
		case SetEntryPointMaxLevel:
			err = a.ReadEP(a.reader)
		case AddLinkAtLevel:
			err = a.ReadLink(a.reader)
		case ReplaceLinksAtLevel:
			err = a.ReadLinks(a.reader)
		case AddTombstone:
			err = a.ReadAddTombstone(a.reader)
		case RemoveTombstone:
			err = a.ReadRemoveTombstone(a.reader)
		case ClearLinks:
			err = a.ReadClearLinks(a.reader)
		case DeleteNode:
			err = a.ReadDeleteNode(a.reader)
		case ResetIndex:
			a.index.nodes = make([]mmapIndexNode, 0, 10000)
		default:
			err = errors.Errorf("unrecognized commit type %d", ct)
		}
		if err != nil {
			// do not return nil, err, because the err could be a recoverable one
			return err
		}
	}

	return nil
}

func (a *MmapCondensorAnalyzer) ReadNode(r io.Reader) error {
	id, err := a.readUint64(r)
	if err != nil {
		return err
	}

	level, err := a.readUint16(r)
	if err != nil {
		return err
	}

	a.index.UpsertNodeMaxLevel(id, level)
	return nil
}

func (a *MmapCondensorAnalyzer) ReadEP(r io.Reader) error {
	// TODO: is this an issue because of bufio Read vs ReadFull?
	_, err := io.CopyN(io.Discard, r, 10)
	return err
}

func (a *MmapCondensorAnalyzer) ReadLink(r io.Reader) error {
	source, err := a.readUint64(r)
	if err != nil {
		return err
	}

	level, err := a.readUint16(r)
	if err != nil {
		return err
	}

	// TODO: is this an issue because of bufio Read vs ReadFull?
	_, err = io.CopyN(io.Discard, r, 8)
	if err != nil {
		return err
	}
	a.index.UpsertNodeMaxLevel(source, level)

	return nil
}

func (a *MmapCondensorAnalyzer) ReadLinks(r io.Reader) error {
	source, err := a.readUint64(r)
	if err != nil {
		return err
	}

	level, err := a.readUint16(r)
	if err != nil {
		return err
	}

	length, err := a.readUint16(r)
	if err != nil {
		return err
	}

	a.index.UpsertNodeMaxLevel(source, level)

	// TODO: is this an issue because of bufio Read vs ReadFull?
	_, err = io.CopyN(io.Discard, r, 8*int64(length))
	if err != nil {
		return err
	}

	return nil
}

func (a *MmapCondensorAnalyzer) ReadAddTombstone(r io.Reader) error {
	// TODO: is this an issue because of bufio Read vs ReadFull?
	_, err := io.CopyN(io.Discard, r, 8)
	return err
}

func (a *MmapCondensorAnalyzer) ReadRemoveTombstone(r io.Reader) error {
	// TODO: is this an issue because of bufio Read vs ReadFull?
	_, err := io.CopyN(io.Discard, r, 8)
	return err
}

func (a *MmapCondensorAnalyzer) ReadClearLinks(r io.Reader) error {
	// TODO: is this an issue because of bufio Read vs ReadFull?
	_, err := io.CopyN(io.Discard, r, 8)
	return err
}

func (a *MmapCondensorAnalyzer) ReadDeleteNode(r io.Reader) error {
	id, err := a.readUint64(r)
	if err != nil {
		return err
	}

	a.index.DeleteNode(id)
	return nil
}

func (a *MmapCondensorAnalyzer) readUint64(r io.Reader) (uint64, error) {
	var value uint64
	tmpBuf := make([]byte, 8)
	_, err := io.ReadFull(r, tmpBuf)
	if err != nil {
		return 0, errors.Wrap(err, "failed to read uint64")
	}

	value = binary.LittleEndian.Uint64(tmpBuf)

	return value, nil
}

func (a *MmapCondensorAnalyzer) readUint16(r io.Reader) (uint16, error) {
	var value uint16
	tmpBuf := make([]byte, 2)
	_, err := io.ReadFull(r, tmpBuf)
	if err != nil {
		return 0, errors.Wrap(err, "failed to read uint16")
	}

	value = binary.LittleEndian.Uint16(tmpBuf)

	return value, nil
}

func (a *MmapCondensorAnalyzer) ReadCommitType(r io.Reader) (HnswCommitType, error) {
	tmpBuf := make([]byte, 1)
	if _, err := io.ReadFull(r, tmpBuf); err != nil {
		return 0, errors.Wrap(err, "failed to read commit type")
	}

	return HnswCommitType(tmpBuf[0]), nil
}
