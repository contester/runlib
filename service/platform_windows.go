package service

import (
	"runlib/win32"
)

type PlatformData struct {
	DesktopName string
	Desktop     win32.Hdesk
}
