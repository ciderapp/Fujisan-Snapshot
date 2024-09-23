//go:build windows

package main

import (
	"errors"
	"fmt"
	"golang.org/x/sys/windows/registry"
	"os"
)

// RegisterCallbackUrl registers CURRENT_USER/Software/Classes for `fujisan`, `cider`, `itms`, `itmss`, `music`, and `musics`. It creates URL Protocol links to the last position of the binary opened
func (c *Cider) RegisterCallbackUrl() error {
	if c.InDevMode() {
		return nil
	}
	aliases := []string{"fujisan", "cider", "itms", "itmss", "music", "musics"}
	for _, alias := range aliases {
		key, _, err := registry.CreateKey(registry.CURRENT_USER, "Software\\Classes\\"+alias, registry.WRITE)
		if err != nil {
			return errors.New("error opening registry key: " + err.Error())
		}
		defer key.Close()

		err = key.SetStringValue("", fmt.Sprintf("URL:%s Protocol", alias))
		if err != nil {
			return errors.New("error setting key value: " + err.Error())
		}

		err = key.SetStringValue("URL Protocol", "")
		if err != nil {
			return errors.New("error setting key value: " + err.Error())
		}

		key, _, err = registry.CreateKey(key, "shell\\open\\command", registry.WRITE)
		if err != nil {
			return errors.New("error crating subkey: " + err.Error())
		}
		defer key.Close()

		path, _ := os.Executable()

		err = key.SetStringValue("", fmt.Sprintf("\"%s\" \"%%1\"", path))
		if err != nil {
			return errors.New("error setting key value: " + err.Error())
		}
	}
	return nil
}
