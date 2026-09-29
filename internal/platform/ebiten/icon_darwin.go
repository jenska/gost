package ebiten

import (
	"unsafe"

	"github.com/ebitengine/purego/objc"
)

func setDockIcon(png []byte) {
	if len(png) == 0 {
		return
	}
	data := objc.ID(objc.GetClass("NSData")).Send(objc.RegisterName("dataWithBytes:length:"),
		unsafe.Pointer(&png[0]), uint64(len(png)))
	icon := objc.ID(objc.GetClass("NSImage")).Send(objc.RegisterName("alloc")).
		Send(objc.RegisterName("initWithData:"), data)
	if icon == 0 {
		return
	}
	app := objc.ID(objc.GetClass("NSApplication")).Send(objc.RegisterName("sharedApplication"))
	app.Send(objc.RegisterName("setApplicationIconImage:"), icon)
}
