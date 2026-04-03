package emulator

import (
	"encoding/binary"
	"fmt"
	"os"
)

const msaMagic = 0x0E0F

func LoadDiskImage(path string) ([]byte, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if !looksLikeMSA(data) {
		return data, nil
	}
	return decodeMSA(data)
}

func looksLikeMSA(data []byte) bool {
	return len(data) >= 10 && binary.BigEndian.Uint16(data[:2]) == msaMagic
}

func decodeMSA(data []byte) ([]byte, error) {
	if len(data) < 10 {
		return nil, fmt.Errorf("MSA image too short")
	}

	sectorsPerTrack := int(binary.BigEndian.Uint16(data[2:4]))
	sides := int(binary.BigEndian.Uint16(data[4:6])) + 1
	startTrack := int(binary.BigEndian.Uint16(data[6:8]))
	endTrack := int(binary.BigEndian.Uint16(data[8:10]))
	if sectorsPerTrack <= 0 || sides <= 0 || endTrack < startTrack {
		return nil, fmt.Errorf("invalid MSA header")
	}

	trackSize := sectorsPerTrack * 512
	out := make([]byte, 0, (endTrack-startTrack+1)*sides*trackSize)
	pos := 10
	for track := startTrack; track <= endTrack; track++ {
		for side := 0; side < sides; side++ {
			if pos+2 > len(data) {
				return nil, fmt.Errorf("unexpected end of MSA image")
			}
			blockLen := int(binary.BigEndian.Uint16(data[pos : pos+2]))
			pos += 2
			if pos+blockLen > len(data) {
				return nil, fmt.Errorf("truncated MSA track payload")
			}

			if blockLen == trackSize {
				out = append(out, data[pos:pos+blockLen]...)
				pos += blockLen
				continue
			}

			end := pos + blockLen
			trackData := make([]byte, 0, trackSize)
			for pos < end {
				b := data[pos]
				pos++
				if b != 0xE5 {
					trackData = append(trackData, b)
					continue
				}
				if pos+3 > end {
					return nil, fmt.Errorf("truncated MSA RLE sequence")
				}
				value := data[pos]
				count := int(binary.BigEndian.Uint16(data[pos+1 : pos+3]))
				pos += 3
				for i := 0; i < count; i++ {
					trackData = append(trackData, value)
				}
			}
			if len(trackData) != trackSize {
				return nil, fmt.Errorf("decoded MSA track has %d bytes, want %d", len(trackData), trackSize)
			}
			out = append(out, trackData...)
		}
	}

	return out, nil
}
