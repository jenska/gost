//go:build darwin && cgo

package host

/*
#cgo CFLAGS: -x objective-c -fblocks
#cgo LDFLAGS: -framework AppKit -framework Foundation -framework UniformTypeIdentifiers

#import <AppKit/AppKit.h>
#import <UniformTypeIdentifiers/UniformTypeIdentifiers.h>
#include <stdlib.h>

static char* gostSelectFloppyDiskImage(void) {
	@autoreleasepool {
		NSApplication *app = [NSApplication sharedApplication];
		[app activateIgnoringOtherApps:YES];

		NSOpenPanel *panel = [NSOpenPanel openPanel];
		[panel setTitle:@"Select a floppy disk image"];
		[panel setPrompt:@"Select"];
		[panel setCanChooseFiles:YES];
		[panel setCanChooseDirectories:NO];
		[panel setAllowsMultipleSelection:NO];
		NSMutableArray<UTType *> *contentTypes = [NSMutableArray arrayWithCapacity:5];
		for (NSString *extension in @[@"st", @"msa", @"stx", @"dim", @"adi"]) {
			UTType *type = [UTType typeWithFilenameExtension:extension];
			if (type != nil) {
				[contentTypes addObject:type];
			}
		}
		[panel setAllowedContentTypes:contentTypes];
		[panel setAllowsOtherFileTypes:NO];
		[panel setLevel:NSModalPanelWindowLevel];
		[panel makeKeyAndOrderFront:nil];

		if ([panel runModal] == NSModalResponseOK) {
			NSURL *url = [panel URL];
			NSString *path = [url path];
			if (path != nil) {
				return strdup([path UTF8String]);
			}
		}
	}
	return NULL;
}
*/
import "C"

import (
	"errors"
	"fmt"
	"runtime"
	"strings"
	"unsafe"
)

func SelectFloppyDiskImage() (string, error) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	selected := C.gostSelectFloppyDiskImage()
	if selected == nil {
		return "", ErrFileDialogCanceled
	}
	defer C.free(unsafe.Pointer(selected))

	path := strings.TrimSpace(C.GoString(selected))
	if err := ValidateFloppyDiskImagePath(path); err != nil {
		if errors.Is(err, ErrFileDialogCanceled) {
			return "", err
		}
		return "", fmt.Errorf("%w; supported extensions are .st, .msa, .stx, .dim, and .adi", err)
	}
	return path, nil
}
