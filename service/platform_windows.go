package service

import (
	"runlib/platform/win32"
)

type PlatformData struct {
	DesktopName string
	Desktop     win32.Hdesk
}
