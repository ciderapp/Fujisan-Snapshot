package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"

	"github.com/kirsle/configdir"
)

// IO is the filesystem interaction class for the frontend to read and write file on the system with very little overhead
type IO struct {
	configPath string
}

// NewIO returns `*IO`
func NewIO() *IO {
	return &IO{}
}

// ReadFile reads a file relative to the ConfigPath normally located at `%APPDATA%/Roaming/Cider-Fuji`
func (i *IO) ReadFile(filename string) string {
	i.GetConfigPath()
	filePath := filepath.Join(i.configPath, filename)
	if !i.FileExists(filePath) {
		return ""
	}
	file, err := os.ReadFile(filePath)
	if err != nil {
		log.Println("Error reading:", filename)
		return ""
	}
	return string(file)
}

// WriteFile writes a file relative to the ConfigPath normally located at `%APPDATA%/Roaming/Cider-Fuji`
func (i *IO) WriteFile(filename string, data string) bool {
	i.GetConfigPath()
	filePath := filepath.Join(i.configPath, filename)
	file, err := os.Create(filePath)
	if err != nil {
		return false
	}
	defer file.Close()

	if filepath.Ext(filename) == ".json" {
		var temp map[string]interface{}
		_ = json.Unmarshal([]byte(data), &temp)
		dataNew, _ := json.MarshalIndent(temp, "", "\t")
		data = string(dataNew)
	}

	if _, err = file.Write([]byte(data)); err != nil {
		log.Println("Error writing:", filename)
		return false
	}
	return true
}

// RemoveFile removes a file relative to the ConfigPath normally located at `%APPDATA%/Roaming/Cider-Fuji`
func (i *IO) RemoveFile(filename string) bool {
	i.GetConfigPath()
	filePath := filepath.Join(i.configPath, filename)
	log.Println("Removing:", filePath)
	if err := os.Remove(filePath); err != nil {
		return false
	}
	return true
}

// GetConfigPath returns the AppData config location for FujisanObject
func (i *IO) GetConfigPath() string {
	i.configPath = configdir.LocalConfig("Cider-Fuji")
	if err := configdir.MakePath(i.configPath); err != nil {
		fmt.Println("Failed to make configuration directory")
		return ""
	}
	return i.configPath
}

// FileExists is able to check if any file exists in the entire filesystem
func (i *IO) FileExists(filename string) bool {
	_, err := os.Stat(filename)
	return !errors.Is(err, fs.ErrNotExist)
}
