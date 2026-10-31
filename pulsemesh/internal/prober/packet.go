// Package prober implements the PulseMesh UDP probe engine and its binary wire
// protocol.
package prober

import (
	"encoding/binary"
	"errors"
	"fmt"
	"time"
)

// Wire-protocol constants.
const (
	// MagicBytes identifies a PulseMesh probe packet ("PM").
	MagicBytes uint16 = 0x504D

	// Version is the current protocol version.
	Version uint8 = 1

	// MsgTypePing is a probe request.
	MsgTypePing uint8 = 1

	// MsgTypePong is a probe response.
	MsgTypePong uint8 = 2

	// PacketSize is the total wire size of a serialized Packet.
	PacketSize = 2 + 1 + 1 + 8 + 8 + 16 // 36 bytes
)

// Packet is the PulseMesh probe packet transmitted over UDP.
//
//	Offset  Size  Field
//	0       2     Magic (0x504D)
//	2       1     Version
//	3       1     MsgType (1=Ping, 2=Pong)
//	4       8     SequenceNum
//	12      8     OriginTimestampNano (monotonic nanoseconds)
//	20      16    NodeID
//	------- 36 bytes total -------
type Packet struct {
	Magic              uint16
	Version            uint8
	MsgType            uint8
	SequenceNum        uint64
	OriginTimestampNano int64
	NodeID             [16]byte
}

// NewPing creates a Ping packet with the given sequence number, node ID, and
// the current monotonic timestamp.
func NewPing(seq uint64, nodeID [16]byte) Packet {
	return Packet{
		Magic:              MagicBytes,
		Version:            Version,
		MsgType:            MsgTypePing,
		SequenceNum:        seq,
		OriginTimestampNano: time.Now().UnixNano(),
		NodeID:             nodeID,
	}
}

// NewPong creates a Pong packet in response to the given ping, preserving its
// OriginTimestampNano and SequenceNum so the sender can compute RTT.
func NewPong(ping Packet, nodeID [16]byte) Packet {
	return Packet{
		Magic:              MagicBytes,
		Version:            Version,
		MsgType:            MsgTypePong,
		SequenceNum:        ping.SequenceNum,
		OriginTimestampNano: ping.OriginTimestampNano,
		NodeID:             nodeID,
	}
}

// Marshal serializes a Packet into a 36-byte big-endian wire format.
func Marshal(p Packet) ([]byte, error) {
	buf := make([]byte, PacketSize)
	binary.BigEndian.PutUint16(buf[0:2], p.Magic)
	buf[2] = p.Version
	buf[3] = p.MsgType
	binary.BigEndian.PutUint64(buf[4:12], p.SequenceNum)
	binary.BigEndian.PutUint64(buf[12:20], uint64(p.OriginTimestampNano))
	copy(buf[20:36], p.NodeID[:])
	return buf, nil
}

// Unmarshal deserializes a 36-byte buffer into a Packet, validating the magic
// bytes and version.
func Unmarshal(data []byte) (Packet, error) {
	if len(data) < PacketSize {
		return Packet{}, fmt.Errorf("packet too short: got %d bytes, want %d", len(data), PacketSize)
	}

	magic := binary.BigEndian.Uint16(data[0:2])
	if magic != MagicBytes {
		return Packet{}, errors.New("invalid magic bytes")
	}

	ver := data[2]
	if ver != Version {
		return Packet{}, fmt.Errorf("unsupported version %d", ver)
	}

	var p Packet
	p.Magic = magic
	p.Version = ver
	p.MsgType = data[3]
	p.SequenceNum = binary.BigEndian.Uint64(data[4:12])
	p.OriginTimestampNano = int64(binary.BigEndian.Uint64(data[12:20]))
	copy(p.NodeID[:], data[20:36])
	return p, nil
}
