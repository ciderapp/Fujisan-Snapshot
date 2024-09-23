//go:build windows

package main

import (
	"golang.org/x/sys/windows"
	"log"
)

func GetVersion() uint32 {
	version := windows.RtlGetVersion()
	log.Println(version.BuildNumber)
	return version.BuildNumber
}
