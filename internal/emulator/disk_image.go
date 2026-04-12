package emulator

import (
	"encoding/binary"
	"fmt"
	"os"
)

const msaMagic = 0x0E0F

type DiskGeometry struct {
	SectorsPerTrack int
	Sides           int
	Tracks          int
}

type DiskImage struct {
	Data     []byte
	Geometry DiskGeometry
}

func NewDiskImage(data []byte) *DiskImage {
	cloned := append([]byte(nil), data...)
	return &DiskImage{
		Data:     cloned,
		Geometry: inferDiskGeometry(cloned),
	}
}

func LoadDiskImage(path string) (*DiskImage, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if !looksLikeMSA(data) {
		return NewDiskImage(data), nil
	}
	return decodeMSA(data)
}

func looksLikeMSA(data []byte) bool {
	return len(data) >= 10 && binary.BigEndian.Uint16(data[:2]) == msaMagic
}

func decodeMSA(data []byte) (*DiskImage, error) {
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
		for range sides {
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
				for range count {
					trackData = append(trackData, value)
				}
			}
			if len(trackData) != trackSize {
				return nil, fmt.Errorf("decoded MSA track has %d bytes, want %d", len(trackData), trackSize)
			}
			out = append(out, trackData...)
		}
	}

	return &DiskImage{
		Data: out,
		Geometry: DiskGeometry{
			SectorsPerTrack: sectorsPerTrack,
			Sides:           sides,
			Tracks:          endTrack - startTrack + 1,
		},
	}, nil
}

func inferDiskGeometry(data []byte) DiskGeometry {
	if len(data) == 0 || len(data)%512 != 0 {
		return DiskGeometry{}
	}

	type candidate struct {
		geometry DiskGeometry
		score    int
	}

	best := candidate{}
	for _, sectorsPerTrack := range []int{9, 10, 11, 18} {
		for _, sides := range []int{2, 1} {
			bytesPerTrack := sectorsPerTrack * sides * 512
			if bytesPerTrack == 0 || len(data)%bytesPerTrack != 0 {
				continue
			}
			tracks := len(data) / bytesPerTrack
			if tracks <= 0 || tracks > 255 {
				continue
			}

			score := 0
			if tracks == 80 {
				score += 100
			}
			if sides == 2 {
				score += 10
			}
			score -= absInt(tracks - 80)
			if score > best.score {
				best = candidate{
					geometry: DiskGeometry{
						SectorsPerTrack: sectorsPerTrack,
						Sides:           sides,
						Tracks:          tracks,
					},
					score: score,
				}
			}
		}
	}

	if best.geometry.SectorsPerTrack != 0 {
		return best.geometry
	}

	return DiskGeometry{
		SectorsPerTrack: len(data) / 512,
		Sides:           1,
		Tracks:          1,
	}
}

func absInt(value int) int {
	if value < 0 {
		return -value
	}
	return value
}
