package emulator

import (
	"fmt"
	"os"
)

// romImageMaxBytes is a generous ceiling on a loadable ROM image. Real TOS
// images top out at 512 KiB and the ST ROM window spans 1 MiB; anything larger
// is almost certainly the wrong file (a disk or hard-disk image passed as
// --rom or --cartridge).
const romImageMaxBytes = 1024 * 1024

// tosImageSizes are the sizes real TOS ROM images come in (192, 256, 512 KiB).
var tosImageSizes = []int{192 * 1024, 256 * 1024, 512 * 1024}

// truncatedTOSImage reports the standard TOS image size that size falls one
// byte short of. Dumps are sometimes truncated that way; loadROMImage pads the
// missing byte with $FF, which can silently corrupt a pointer stored at the
// very end of the ROM (TOS 1.02 keeps its desktop start address there).
func truncatedTOSImage(size int) (want int, truncated bool) {
	for _, s := range tosImageSizes {
		if size == s-1 {
			return s, true
		}
	}
	return 0, false
}

// loadROMImage reads a TOS or cartridge ROM image, rejecting empty and
// oversized files and padding an odd length to a whole word with $FF.
func loadROMImage(path string) ([]byte, error) {
	image, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if len(image) == 0 {
		return nil, fmt.Errorf("ROM image %q is empty", path)
	}
	if len(image) > romImageMaxBytes {
		return nil, fmt.Errorf("ROM image %q is %d bytes, which exceeds the %d byte maximum", path, len(image), romImageMaxBytes)
	}
	if len(image)%2 != 0 {
		image = append(image, 0xFF)
	}
	return image, nil
}
