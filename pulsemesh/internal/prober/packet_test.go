package prober

import (
	"bytes"
	"testing"
)

func TestMarshalUnmarshalRoundTrip(t *testing.T) {
	nodeID := [16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}
	ping := NewPing(42, nodeID)

	data, err := Marshal(ping)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if len(data) != PacketSize {
		t.Fatalf("Marshal produced %d bytes, want %d", len(data), PacketSize)
	}

	got, err := Unmarshal(data)
	if err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	if got.Magic != MagicBytes {
		t.Errorf("Magic = 0x%X, want 0x%X", got.Magic, MagicBytes)
	}
	if got.Version != Version {
		t.Errorf("Version = %d, want %d", got.Version, Version)
	}
	if got.MsgType != MsgTypePing {
		t.Errorf("MsgType = %d, want %d", got.MsgType, MsgTypePing)
	}
	if got.SequenceNum != 42 {
		t.Errorf("SequenceNum = %d, want 42", got.SequenceNum)
	}
	if got.OriginTimestampNano != ping.OriginTimestampNano {
		t.Errorf("OriginTimestampNano mismatch")
	}
	if got.NodeID != nodeID {
		t.Errorf("NodeID mismatch")
	}
}

func TestUnmarshalInvalidMagic(t *testing.T) {
	data := make([]byte, PacketSize)
	data[0] = 0xFF
	data[1] = 0xFF
	data[2] = Version

	_, err := Unmarshal(data)
	if err == nil {
		t.Fatal("expected error for invalid magic bytes")
	}
}

func TestUnmarshalShortData(t *testing.T) {
	data := make([]byte, 10) // too short
	_, err := Unmarshal(data)
	if err == nil {
		t.Fatal("expected error for short data")
	}
}

func TestNewPongPreservesFields(t *testing.T) {
	senderID := [16]byte{0xAA}
	responderID := [16]byte{0xBB}

	ping := NewPing(99, senderID)
	pong := NewPong(ping, responderID)

	if pong.MsgType != MsgTypePong {
		t.Errorf("pong MsgType = %d, want %d", pong.MsgType, MsgTypePong)
	}
	if pong.SequenceNum != ping.SequenceNum {
		t.Error("pong SequenceNum should match ping")
	}
	if pong.OriginTimestampNano != ping.OriginTimestampNano {
		t.Error("pong OriginTimestampNano should match ping")
	}
	if !bytes.Equal(pong.NodeID[:], responderID[:]) {
		t.Error("pong NodeID should be the responder's")
	}
}

func TestPongMarshalRoundTrip(t *testing.T) {
	senderID := [16]byte{1}
	responderID := [16]byte{2}

	ping := NewPing(7, senderID)
	pong := NewPong(ping, responderID)

	data, err := Marshal(pong)
	if err != nil {
		t.Fatalf("Marshal pong: %v", err)
	}
	got, err := Unmarshal(data)
	if err != nil {
		t.Fatalf("Unmarshal pong: %v", err)
	}
	if got.MsgType != MsgTypePong {
		t.Errorf("MsgType = %d, want Pong(%d)", got.MsgType, MsgTypePong)
	}
	if got.SequenceNum != 7 {
		t.Errorf("SequenceNum = %d, want 7", got.SequenceNum)
	}
}
