package emulator

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"
)

func TestLoadDiskImageReturnsRawSTBytes(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "disk.st")
	want := make([]byte, 9*80*2*512)
	copy(want[:4], []byte{0xDE, 0xAD, 0xBE, 0xEF})
	if err := os.WriteFile(path, want, 0o644); err != nil {
		t.Fatalf("write raw disk image: %v", err)
	}

	got, err := LoadDiskImage(path)
	if err != nil {
		t.Fatalf("load raw disk image: %v", err)
	}
	if string(got.Data) != string(want) {
		t.Fatalf("unexpected raw disk image bytes: got %x want %x", got.Data[:4], want[:4])
	}
	if got.Geometry.SectorsPerTrack != 9 || got.Geometry.Sides != 2 || got.Geometry.Tracks != 80 {
		t.Fatalf("unexpected raw disk geometry: %+v", got.Geometry)
	}
}

func TestLoadDiskImageDecodesMSA(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "disk.msa")
	msa := []byte{
		0x0E, 0x0F, // magic
		0x00, 0x01, // sectors per track
		0x00, 0x00, // one side
		0x00, 0x00, // start track
		0x00, 0x00, // end track
		0x00, 0x04, // compressed block length
		0xE5, 0x11, 0x02, 0x00, // repeat 0x11 for the full 512-byte track
	}
	if err := os.WriteFile(path, msa, 0o644); err != nil {
		t.Fatalf("write MSA disk image: %v", err)
	}

	got, err := LoadDiskImage(path)
	if err != nil {
		t.Fatalf("load MSA disk image: %v", err)
	}
	if len(got.Data) != 512 {
		t.Fatalf("decoded MSA length = %d, want 512", len(got.Data))
	}
	for i := range len(got.Data) {
		if got.Data[i] != 0x11 {
			t.Fatalf("decoded byte %d = %02x, want 11", i, got.Data[i])
		}
	}
	if got.Geometry.SectorsPerTrack != 1 || got.Geometry.Sides != 1 || got.Geometry.Tracks != 1 {
		t.Fatalf("unexpected MSA geometry: %+v", got.Geometry)
	}
}

func TestLoadDiskImagePreservesDoubleSidedMSAGeometry(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "double-sided.msa")
	msa := []byte{
		0x0E, 0x0F, // magic
		0x00, 0x01, // sectors per track
		0x00, 0x01, // two sides
		0x00, 0x00, // start track
		0x00, 0x00, // end track
		0x00, 0x04, // side 0 compressed block length
		0xE5, 0x11, 0x02, 0x00,
		0x00, 0x04, // side 1 compressed block length
		0xE5, 0x22, 0x02, 0x00,
	}
	if err := os.WriteFile(path, msa, 0o644); err != nil {
		t.Fatalf("write MSA disk image: %v", err)
	}

	got, err := LoadDiskImage(path)
	if err != nil {
		t.Fatalf("load double-sided MSA: %v", err)
	}
	if got.Geometry.SectorsPerTrack != 1 || got.Geometry.Sides != 2 || got.Geometry.Tracks != 1 {
		t.Fatalf("unexpected double-sided MSA geometry: %+v", got.Geometry)
	}
	if len(got.Data) != 1024 {
		t.Fatalf("decoded double-sided MSA length = %d, want 1024", len(got.Data))
	}
	if got.Data[0] != 0x11 || got.Data[512] != 0x22 {
		t.Fatalf("unexpected double-sided data markers: %02x %02x", got.Data[0], got.Data[512])
	}
}

func TestLoadDiskImageDecodesDIMHeaderedSectorImage(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "disk.dim")
	payload := make([]byte, 2*512)
	payload[0] = 0xCA
	payload[512] = 0xFE
	header := make([]byte, dimHeaderSize)
	header[0] = 0x42
	header[1] = 0x42
	header[6] = 1    // two sides
	header[8] = 1    // one sector per track
	header[0x0C] = 0 // track 0 only
	image := append(header, payload...)
	if err := os.WriteFile(path, image, 0o644); err != nil {
		t.Fatalf("write DIM disk image: %v", err)
	}

	got, err := LoadDiskImage(path)
	if err != nil {
		t.Fatalf("load DIM disk image: %v", err)
	}
	if len(got.Data) != len(payload) {
		t.Fatalf("decoded DIM length = %d, want %d", len(got.Data), len(payload))
	}
	if got.Data[0] != 0xCA || got.Data[512] != 0xFE {
		t.Fatalf("unexpected DIM payload markers: %02x %02x", got.Data[0], got.Data[512])
	}
	if got.Geometry.SectorsPerTrack != 1 || got.Geometry.Sides != 2 || got.Geometry.Tracks != 1 {
		t.Fatalf("unexpected DIM geometry: %+v", got.Geometry)
	}
}

func TestLoadDiskImageRejectsCompressedDIM(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "compressed.adi")
	header := make([]byte, dimHeaderSize)
	header[0] = 0x42
	header[1] = 0x42
	header[3] = 1
	header[6] = 0
	header[8] = 1
	if err := os.WriteFile(path, header, 0o644); err != nil {
		t.Fatalf("write compressed DIM disk image: %v", err)
	}

	if _, err := LoadDiskImage(path); err == nil {
		t.Fatalf("expected compressed DIM-style image to be rejected")
	}
}

func TestLoadDiskImageDecodesSTXStandardSectors(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "disk.stx")
	sector1 := make([]byte, 512)
	sector2 := make([]byte, 512)
	sector1[0] = 0x11
	sector2[0] = 0x22
	image := stxTestHeader(1)
	track := make([]byte, 16)
	binary.LittleEndian.PutUint32(track[0:4], uint32(16+len(sector1)+len(sector2)))
	binary.LittleEndian.PutUint16(track[8:10], 2)
	image = append(image, track...)
	image = append(image, sector1...)
	image = append(image, sector2...)
	if err := os.WriteFile(path, image, 0o644); err != nil {
		t.Fatalf("write STX disk image: %v", err)
	}

	got, err := LoadDiskImage(path)
	if err != nil {
		t.Fatalf("load STX disk image: %v", err)
	}
	if got.Geometry.SectorsPerTrack != 2 || got.Geometry.Sides != 1 || got.Geometry.Tracks != 1 {
		t.Fatalf("unexpected STX geometry: %+v", got.Geometry)
	}
	if len(got.Data) != 1024 {
		t.Fatalf("decoded STX length = %d, want 1024", len(got.Data))
	}
	if got.Data[0] != 0x11 || got.Data[512] != 0x22 {
		t.Fatalf("unexpected STX sector markers: %02x %02x", got.Data[0], got.Data[512])
	}
}

func TestLoadDiskImageDecodesSTXDescriptorTrackImageSectors(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "disk.stx")
	sector := make([]byte, 512)
	sector[0] = 0xCA
	sector[511] = 0xFE
	trackImage := append([]byte{0x02, 0x00}, sector...)
	image := stxTestHeader(1)
	track := make([]byte, 16)
	binary.LittleEndian.PutUint32(track[0:4], uint32(16+16+len(trackImage)))
	binary.LittleEndian.PutUint16(track[8:10], 1)
	binary.LittleEndian.PutUint16(track[10:12], stxTrackFlagSectorDescriptors|stxTrackFlagImage)
	image = append(image, track...)
	desc := make([]byte, 16)
	binary.LittleEndian.PutUint32(desc[0:4], 2)
	desc[8] = 0
	desc[9] = 0
	desc[10] = 1
	desc[11] = stxSectorSizeCode512
	image = append(image, desc...)
	image = append(image, trackImage...)
	if err := os.WriteFile(path, image, 0o644); err != nil {
		t.Fatalf("write STX disk image: %v", err)
	}

	got, err := LoadDiskImage(path)
	if err != nil {
		t.Fatalf("load STX disk image: %v", err)
	}
	if got.Geometry.SectorsPerTrack != 1 || got.Geometry.Sides != 1 || got.Geometry.Tracks != 1 {
		t.Fatalf("unexpected STX geometry: %+v", got.Geometry)
	}
	if len(got.Data) != 512 {
		t.Fatalf("decoded STX length = %d, want 512", len(got.Data))
	}
	if got.Data[0] != 0xCA || got.Data[511] != 0xFE {
		t.Fatalf("unexpected STX data markers: %02x %02x", got.Data[0], got.Data[511])
	}
}

func TestLoadDiskImageDecodesLocalAtarimaniaSTX(t *testing.T) {
	path := filepath.Join("..", "..", "FLOPPIES", "1st_word_plus_2.02_disk_1_1986_gst_software.stx")
	if _, err := os.Stat(path); err != nil {
		t.Skipf("local STX fixture not available: %v", err)
	}

	got, err := LoadDiskImage(path)
	if err != nil {
		t.Fatalf("load local STX disk image: %v", err)
	}
	if got.Geometry.SectorsPerTrack != 9 || got.Geometry.Sides != 1 || got.Geometry.Tracks != 80 {
		t.Fatalf("unexpected local STX geometry: %+v", got.Geometry)
	}
	if len(got.Data) != 9*80*512 {
		t.Fatalf("decoded local STX length = %d, want %d", len(got.Data), 9*80*512)
	}
	if got.Data[8] != 0x94 || got.Data[9] != 0xE0 {
		t.Fatalf("unexpected local STX boot marker: %02x %02x", got.Data[8], got.Data[9])
	}
}

func TestLoadDiskImageRejectsSCPFluxImages(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "disk.scp")
	if err := os.WriteFile(path, []byte{'S', 'C', 'P', 0x19}, 0o644); err != nil {
		t.Fatalf("write SCP disk image: %v", err)
	}

	if _, err := LoadDiskImage(path); err == nil {
		t.Fatalf("expected SCP flux image to be rejected")
	}
}

func stxTestHeader(trackCount byte) []byte {
	header := make([]byte, 16)
	copy(header[:4], []byte{'R', 'S', 'Y', 0})
	binary.LittleEndian.PutUint16(header[4:6], 3)
	header[0x0A] = trackCount
	header[0x0B] = 2
	return header
}
