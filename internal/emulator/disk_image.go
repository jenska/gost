package emulator

import (
	"encoding/binary"
	"fmt"
	"os"
)

const msaMagic = 0x0E0F
const (
	dimHeaderSize = 32
	stxHeaderSize = 16
)

const (
	stxTrackFlagSectorDescriptors = 0x01
	stxTrackFlagImage             = 0x40
	stxTrackFlagSyncOffset        = 0x80
	stxSectorFlagRecordNotFound   = 0x10
	stxSectorSizeCode512          = 2
)

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
	if looksLikeSTX(data) {
		return decodeSTX(data)
	}
	if looksLikeSCP(data) {
		return nil, fmt.Errorf("SCP flux images are not supported yet; use a .stx, .st, .msa, .dim, or compatible .adi image")
	}
	if !looksLikeMSA(data) {
		if looksLikeDIM(data) {
			return decodeDIM(data)
		}
		return NewDiskImage(data), nil
	}
	return decodeMSA(data)
}

func looksLikeMSA(data []byte) bool {
	return len(data) >= 10 && binary.BigEndian.Uint16(data[:2]) == msaMagic
}

func looksLikeDIM(data []byte) bool {
	return len(data) >= dimHeaderSize &&
		data[0] == 0x42 &&
		data[1] == 0x42 &&
		data[3] <= 1 &&
		data[6] <= 1 &&
		data[8] > 0 &&
		data[0x0A] == 0
}

func looksLikeSTX(data []byte) bool {
	return len(data) >= stxHeaderSize &&
		data[0] == 'R' &&
		data[1] == 'S' &&
		data[2] == 'Y' &&
		data[3] == 0
}

func looksLikeSCP(data []byte) bool {
	return len(data) >= 3 &&
		data[0] == 'S' &&
		data[1] == 'C' &&
		data[2] == 'P'
}

func decodeDIM(data []byte) (*DiskImage, error) {
	if len(data) < dimHeaderSize {
		return nil, fmt.Errorf("DIM image too short")
	}
	if data[3] != 0 {
		return nil, fmt.Errorf("compressed DIM images are not supported")
	}

	sectorsPerTrack := int(data[8])
	sides := int(data[6]) + 1
	startTrack := int(data[0x0A])
	endTrack := int(data[0x0C])
	if sectorsPerTrack <= 0 || sides <= 0 || endTrack < startTrack {
		return nil, fmt.Errorf("invalid DIM header")
	}

	payload := data[dimHeaderSize:]
	expected := (endTrack - startTrack + 1) * sides * sectorsPerTrack * 512
	if len(payload) != expected {
		return nil, fmt.Errorf("DIM payload has %d bytes, want %d", len(payload), expected)
	}

	return &DiskImage{
		Data: append([]byte(nil), payload...),
		Geometry: DiskGeometry{
			SectorsPerTrack: sectorsPerTrack,
			Sides:           sides,
			Tracks:          endTrack - startTrack + 1,
		},
	}, nil
}

type stxSectorKey struct {
	track  int
	side   int
	sector int
}

type stxSectorDescriptor struct {
	dataOffset uint32
	track      byte
	head       byte
	sector     byte
	sizeCode   byte
	fdcFlags   byte
}

func decodeSTX(data []byte) (*DiskImage, error) {
	if len(data) < stxHeaderSize {
		return nil, fmt.Errorf("STX image too short")
	}
	if !looksLikeSTX(data) {
		return nil, fmt.Errorf("invalid STX file identifier")
	}
	version := binary.LittleEndian.Uint16(data[4:6])
	if version != 3 {
		return nil, fmt.Errorf("unsupported STX version %d", version)
	}

	trackCount := int(data[0x0A])
	pos := stxHeaderSize
	sectors := make(map[stxSectorKey][]byte)
	maxTrack, maxSide, maxSector := -1, -1, 0
	for trackRecord := range trackCount {
		if pos+stxHeaderSize > len(data) {
			return nil, fmt.Errorf("unexpected end of STX image before track %d", trackRecord)
		}
		recordStart := pos
		recordSize := int(binary.LittleEndian.Uint32(data[pos : pos+4]))
		fuzzyCount := int(binary.LittleEndian.Uint32(data[pos+4 : pos+8]))
		sectorCount := int(binary.LittleEndian.Uint16(data[pos+8 : pos+10]))
		trackFlags := binary.LittleEndian.Uint16(data[pos+10 : pos+12])
		trackNumber := data[pos+14]
		recordEnd := recordStart + recordSize
		if recordSize < stxHeaderSize || recordEnd < recordStart || recordEnd > len(data) {
			return nil, fmt.Errorf("invalid STX track %d record size %d", trackRecord, recordSize)
		}

		track := int(trackNumber & 0x7F)
		side := int(trackNumber >> 7)
		cursor := recordStart + stxHeaderSize
		if trackFlags&stxTrackFlagSectorDescriptors == 0 {
			if fuzzyCount != 0 {
				return nil, fmt.Errorf("STX track %d has fuzzy data without sector descriptors", trackRecord)
			}
			cursor, maxTrack, maxSide, maxSector = decodeSTXStandardTrackData(
				data, cursor, recordEnd, sectors, track, side, sectorCount, maxTrack, maxSide, maxSector)
			if cursor < 0 {
				return nil, fmt.Errorf("truncated STX standard sector data in track %d", trackRecord)
			}
			pos = recordEnd
			continue
		}

		descriptors, err := decodeSTXSectorDescriptors(data, cursor, recordEnd, sectorCount)
		if err != nil {
			return nil, fmt.Errorf("decode STX sector descriptors for track %d: %w", trackRecord, err)
		}
		cursor += sectorCount * stxHeaderSize
		if cursor+fuzzyCount > recordEnd {
			return nil, fmt.Errorf("truncated STX fuzzy mask in track %d", trackRecord)
		}
		dataStart := cursor + fuzzyCount

		if trackFlags&stxTrackFlagImage != 0 {
			headerSize := 2
			if trackFlags&stxTrackFlagSyncOffset != 0 {
				headerSize = 4
			}
			if dataStart+headerSize > recordEnd {
				return nil, fmt.Errorf("truncated STX track image header in track %d", trackRecord)
			}
		}

		for _, desc := range descriptors {
			if desc.fdcFlags&stxSectorFlagRecordNotFound != 0 || desc.sizeCode != stxSectorSizeCode512 {
				continue
			}
			start := dataStart + int(desc.dataOffset)
			if start+512 > recordEnd {
				return nil, fmt.Errorf("STX sector %d/%d/%d points outside track record", track, side, desc.sector)
			}
			sector := int(desc.sector)
			if sector <= 0 {
				continue
			}
			key := stxSectorKey{track: track, side: side, sector: sector}
			if _, exists := sectors[key]; exists {
				continue
			}
			sectors[key] = append([]byte(nil), data[start:start+512]...)
			maxTrack = max(maxTrack, track)
			maxSide = max(maxSide, side)
			maxSector = max(maxSector, sector)
		}
		pos = recordEnd
	}
	if len(sectors) == 0 {
		return nil, fmt.Errorf("STX image does not contain supported 512-byte sectors")
	}

	out := make([]byte, (maxTrack+1)*(maxSide+1)*maxSector*512)
	for key, sectorData := range sectors {
		offset := ((key.track*(maxSide+1) + key.side) * maxSector) + (key.sector - 1)
		copy(out[offset*512:], sectorData)
	}
	return &DiskImage{
		Data: out,
		Geometry: DiskGeometry{
			SectorsPerTrack: maxSector,
			Sides:           maxSide + 1,
			Tracks:          maxTrack + 1,
		},
	}, nil
}

func decodeSTXStandardTrackData(data []byte, cursor, recordEnd int, sectors map[stxSectorKey][]byte, track, side, sectorCount, maxTrack, maxSide, maxSector int) (int, int, int, int) {
	for sector := 1; sector <= sectorCount; sector++ {
		if cursor+512 > recordEnd {
			return -1, maxTrack, maxSide, maxSector
		}
		key := stxSectorKey{track: track, side: side, sector: sector}
		sectors[key] = append([]byte(nil), data[cursor:cursor+512]...)
		cursor += 512
		maxTrack = max(maxTrack, track)
		maxSide = max(maxSide, side)
		maxSector = max(maxSector, sector)
	}
	return cursor, maxTrack, maxSide, maxSector
}

func decodeSTXSectorDescriptors(data []byte, cursor, recordEnd, sectorCount int) ([]stxSectorDescriptor, error) {
	if cursor+sectorCount*stxHeaderSize > recordEnd {
		return nil, fmt.Errorf("descriptor table exceeds track record")
	}
	descriptors := make([]stxSectorDescriptor, 0, sectorCount)
	for range sectorCount {
		descriptors = append(descriptors, stxSectorDescriptor{
			dataOffset: binary.LittleEndian.Uint32(data[cursor : cursor+4]),
			track:      data[cursor+8],
			head:       data[cursor+9],
			sector:     data[cursor+10],
			sizeCode:   data[cursor+11],
			fdcFlags:   data[cursor+14],
		})
		cursor += stxHeaderSize
	}
	return descriptors, nil
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
