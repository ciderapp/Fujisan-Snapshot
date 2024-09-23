package main

import (
	"encoding/json"
	"fmt"
	wruntime "github.com/ciderapp/wails/v2/pkg/runtime"
	js "github.com/dop251/goja"
	"github.com/dop251/goja_nodejs/eventloop"
	"github.com/dop251/goja_nodejs/require"
	"github.com/dop251/goja_nodejs/url"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

type Container struct {
	VM        *js.Runtime
	Registry  *require.Registry
	EventLoop *eventloop.EventLoop
	// Watcher  *fsnotify.Watcher
}

type PluginLoader struct {
	PluginFolder string
	VM           map[string]*Container
	Logger       *log.Logger
	Mutex        sync.Mutex
}

func NewPluginLoader(folder string) *PluginLoader {
	logger := log.New(log.Writer(), log.Prefix(), log.Flags())
	logger.SetPrefix("[PluginLoader] ")
	plugin := PluginLoader{
		PluginFolder: folder,
		Logger:       logger,
		VM:           make(map[string]*Container),
	}
	return &plugin
}

func (p *PluginLoader) UnloadPlugin(pluginName string) {
	if p.VM[pluginName] != nil {
		//p.VM[pluginName].Watcher.Close()
		p.VM[pluginName].EventLoop.Stop()
		p.VM[pluginName].VM.Interrupt("halt")
		delete(p.VM, pluginName)
		p.Logger.Println("Unloaded:", pluginName)
	} else {
		p.Logger.Println(pluginName, "already unloaded")
	}
}

func (p *PluginLoader) SetupPluginCalls(loader *PluginLoader, vm *js.Runtime, filename string, pluginName string) {
	vmLogger := log.New(loader.Logger.Writer(), loader.Logger.Prefix(), loader.Logger.Flags())
	vmLogger.SetPrefix(fmt.Sprintf("[%s] ", pluginName))
	vm.Set("print", vmLogger.Print)

	p.VM[pluginName] = new(Container)
	p.VM[pluginName].Registry = require.NewRegistry(require.WithGlobalFolders(filepath.Dir(filename)))
	p.VM[pluginName].EventLoop = eventloop.NewEventLoop()
	p.VM[pluginName].EventLoop.Start()
	p.VM[pluginName].Registry.Enable(vm)
	url.Enable(vm)

	// Expose backend methods to JS
	type ReadableEndpointReturn struct {
		Body   interface{} `json:"body"`
		Status int         `json:"status"`
	}

	vm.Set("musicKit", func(method js.Value, endpoint js.Value, body js.Value) js.Value {
		musicArgs := new(MusicKitArgs)
		musicArgs.Method = method.String()
		musicArgs.Endpoint = endpoint.String()
		musicArgs.Body = body.String()

		ret := new(EndpointReturn)
		FujisanRpcObject.MusicKit(nil, musicArgs, ret)

		var obj map[string]interface{}
		err := json.Unmarshal(ret.Body, &obj)
		if err != nil {
			return nil
		}
		return vm.ToValue(ReadableEndpointReturn{Body: obj, Status: ret.Status})
	})

	vm.Set("alert", func(title js.Value, message js.Value) js.Value {
		wruntime.MessageDialog(FujisanObject.ctx, wruntime.MessageDialogOptions{
			Type:    wruntime.InfoDialog,
			Title:   fmt.Sprintf("%s: %s", filename, title.String()),
			Message: message.String(),
		})
		return nil
	})

	// Pass through IO
	vm.Set("IO", *FujisanIOObject)
}

func (p *PluginLoader) LoadPlugin(filename string, pluginName string) {
	file, err := os.ReadFile(filename)
	if err != nil {
		p.Logger.Println("Unable to load:", filename, err)
		return
	}

	go func(loader *PluginLoader, filename string, pluginName string) {
		formattedName := filepath.Base(filename)

		vm := js.New()
		p.SetupPluginCalls(loader, vm, filename, pluginName)

		loader.Mutex.Lock()
		loader.VM[pluginName].VM = vm
		loader.Mutex.Unlock()

		compile, err := js.Compile(formattedName, string(file), false)
		if err != nil {
			p.Logger.Println(formattedName+":", err)
			return
		}

		p.VM[pluginName].EventLoop.RunOnLoop(func(runtime *js.Runtime) {
			_, err = vm.RunProgram(compile)
			if err != nil {
				p.UnloadPlugin(pluginName)
				loader.Logger.Println(formattedName+":", err)
				return
			}
		})
	}(p, filename, pluginName)

	p.Logger.Println("Loaded:", pluginName)
}

func (p *PluginLoader) ReadPluginMetadata(directory string) (metadata *PluginMetadata) {
	metadata = new(PluginMetadata)
	file, err := os.ReadFile(filepath.Join(directory, "metadata.json"))
	if err != nil {
		p.Logger.Println("Unable to find metadata", err)
		return nil
	}

	if err = json.Unmarshal(file, metadata); err != nil {
		p.Logger.Println("Unable to parse data for", directory, err)
		return nil
	}
	return
}

func (p *PluginLoader) LoadPlugins() {
	if err := os.MkdirAll(p.PluginFolder, os.FileMode(0755)); err != nil {
		return
	}

	filepath.Walk(p.PluginFolder, func(path string, info fs.FileInfo, err error) error {
		if info.IsDir() && info.Name() != "plugins" {
			if metadata := p.ReadPluginMetadata(path); metadata != nil {
				p.Logger.Println("Found program metadata")
				p.Logger.Printf("Name: %v\nVersion: %v\nDescription: %s\nAuthor(s): %v\nFrontend Script: %v\nBackend Script: %v", metadata.Name, metadata.Version, metadata.Description, strings.Join(metadata.Authors, ", "), metadata.FrontendMainScript, metadata.BackendMainScript)
				if len(metadata.BackendMainScript) != 0 {
					p.Logger.Printf("Loading %s into backend", metadata.Name)
					p.LoadPlugin(filepath.Join(path, metadata.BackendMainScript), metadata.Name)
				}

				if len(metadata.FrontendMainScript) != 0 {
					p.Logger.Printf("Loading %s into frontend", metadata.Name)
					file, err := os.ReadFile(filepath.Join(path, metadata.FrontendMainScript))
					if err != nil {
						p.Logger.Println("Unable to load:", metadata.FrontendMainScript, err)
						return nil
					}

					wruntime.WindowExecJS(FujisanObject.ctx, string(file))
				}
			}
		}
		return nil
	})
}
