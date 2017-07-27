package hdhomerun

import (
	"encoding/binary"
	"errors"
	"hash/crc32"
	"io"
)

var (
	// errInvalidChecksum is returned when attempting to unmarshal a Packet
	// with a bad checksum.
	errInvalidChecksum = errors.New("invalid CRC32 checksum")
)

// A Packet is a network packet used to communicate with HDHomeRun devices.
type Packet struct {
	// Type specifies the type of message this Packet carries.
	Type uint16

	// Tags specifies zero or more tags containing optional attributes.
	Tags []Tag
}

// A Tag is an attribute carried by a Packet.
type Tag struct {
	// Type specifies the type of payload this Tag carries.
	Type uint8

	// Data is an arbitrary byte payload.
	Data []byte
}

// MarshalBinary marshals a Packet into its binary form.
func (p *Packet) MarshalBinary() ([]byte, error) {
	// Allocate enough bytes all at once for the Packet.
	var count int
	for _, t := range p.Tags {
		// Tag length may be 2 bytes for larger numbers.
		tlen := 1
		if len(t.Data) > 127 {
			tlen = 2
		}

		count += 1 + tlen + len(t.Data)
	}

	b := make([]byte, 2+2+count+4)

	binary.BigEndian.PutUint16(b[0:2], p.Type)
	binary.BigEndian.PutUint16(b[2:4], uint16(count))

	i := 4
	for _, t := range p.Tags {
		b[i] = t.Type
		i++

		n, err := writeTagLength(len(t.Data), b[i:i+2])
		if err != nil {
			return nil, err
		}
		i += n

		i += copy(b[i:], t.Data)
	}

	chk := crc32.ChecksumIEEE(b[0 : len(b)-4])
	binary.LittleEndian.PutUint32(b[len(b)-4:], chk)

	return b, nil
}

// UnmarshalBinary unmarshals a Packet from its binary form.
func (p *Packet) UnmarshalBinary(b []byte) error {
	// Need enough data for type, tags length, and checksum.
	if len(b) < 8 {
		return io.ErrUnexpectedEOF
	}

	want := binary.LittleEndian.Uint32(b[len(b)-4:])
	got := crc32.ChecksumIEEE(b[0 : len(b)-4])
	if want != got {
		return errInvalidChecksum
	}

	p.Type = binary.BigEndian.Uint16(b[0:2])
	length := int(binary.BigEndian.Uint16(b[2:4]))

	// Don't allow a misleading length value, minus length for
	// type, tags length, and CRC checksum.
	if length != len(b)-8 {
		return io.ErrUnexpectedEOF
	}

	if length == 0 {
		return nil
	}

	p.Tags = make([]Tag, 0)
	for i := 4; i < len(b)-4; {
		t := Tag{
			Type: b[i],
		}
		i++

		tlen, consumed, err := readTagLength(b[i : i+2])
		if err != nil {
			return err
		}
		i += consumed

		// Don't allow a misleading tag length value.
		if len(b[i:])-4 < tlen {
			return io.ErrUnexpectedEOF
		}

		t.Data = make([]byte, len(b[i:i+tlen]))
		copy(t.Data, b[i:i+tlen])
		i += tlen

		p.Tags = append(p.Tags, t)
	}

	return nil
}

// Variable tag length format reading and writing functions as described in:
// https://github.com/Silicondust/libhdhomerun/blob/master/hdhomerun_pkt.h

// TODO(mdlayher): handle large tag lengths properly.

// writeTagLength writes the value of n into b using the variable length tag
// length algorithm used by HDHomeRun devices. It returns the number of bytes
// consumed by the length value.
func writeTagLength(n int, b []byte) (consumed int, err error) {
	if len(b) != 2 {
		return 0, errors.New("must pass exactly two bytes to writeTagLength")
	}

	if n < 128 {
		b[0] = byte(n)
		return 1, nil
	}

	return 0, errors.New("large tags not implemented")
}

// readTagLength reads a length value from b using the variable length tag
// length algorithm used by HDHomeRun devices. It returns the number of bytes
// consumed by the length value.
func readTagLength(b []byte) (length, consumed int, err error) {
	if len(b) != 2 {
		return 0, 0, errors.New("must pass exactly two bytes to readTagLength")
	}

	if b[0]&0x80 == 0 {
		return int(b[0]), 1, nil
	}

	return 0, 0, errors.New("large tags not implemented")

}
