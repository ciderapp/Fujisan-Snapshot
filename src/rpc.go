package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"reflect"
	"strings"
	"time"

	wruntime "github.com/ciderapp/wails/v2/pkg/runtime"
	"github.com/gomarkdown/markdown"
	"github.com/gomarkdown/markdown/html"
	gjson "github.com/gorilla/rpc/json"
)

var (
	JavascriptChannel = make(chan interface{})
)

type (
	// FujisanRpc is the class for doing anything with RPC
	FujisanRpc struct{}
	// RpcType is mainly used for methods that return nothing
	RpcType map[string]interface{}
)

// generateDocsPage Creates an HTML document based on class methods in FujisanRpc that fit the criteria for an RPC function
func (f *FujisanRpc) generateDocsPage() (string, error) {
	// Setup renderer
	htmlFlags := html.CommonFlags | html.HrefTargetBlank | html.CompletePage
	opts := html.RendererOptions{
		Flags:     htmlFlags,
		CSS:       "https://cdn.jsdelivr.net/npm/bootstrap@5.3.0-alpha1/dist/css/bootstrap.min.css",
		Generator: fmt.Sprintf("  <meta name=\"Copyright\" content=\"FujisanObject Rpc, Copyright %s Cider Collective", time.Now().Format("2006")),
	}
	renderer := html.NewRenderer(opts)
	// Create the start of the page in MD
	body := `# FujisanObject Rpc
An RPC (Remote Procedure Call) server for Cider 2(FujisanObject) by freehelpdesk
### How do I call RPC methods?
You send over a POST request to localhost:10782/rpc that contains the following
`
	// Creates a dynamic method call to push to the DOC, used for documentation
	// We need to unmarshal and re marshal to fix formatting
	var temp map[string]interface{}
	marshaled, _ := gjson.EncodeClientRequest("FujisanRpc.SeekTo", SeekToArgs{20})
	if err := json.Unmarshal(marshaled, &temp); err != nil {
		return "", err
	}

	marshaled, _ = json.MarshalIndent(temp, "", "\t")

	body += fmt.Sprintf("```json\n%s\n```\n", string(marshaled))
	body += "### What is `interface {}`?\nThis is the Golang equivalent to a Javascript Object."
	body += `
| Method | Input Parameters | Output Parameters |
|--------|------------------|-------------------|
`

	// Get the type FujisanRpcObject which should be FujisanRpc
	t := reflect.TypeOf(FujisanRpcObject)
	// Enumerate the methods in the struct dynamically
	for i := 0; i < t.NumMethod(); i++ {
		// Get the method at the index of NumMethod
		method := t.Method(i)
		// Make sure to skip over private methods
		if method.IsExported() {
			// Make sure before we add the method, it follows our strict
			// rpc function types, 3 params, and error as output
			if method.Type.NumIn() == 4 && method.Type.NumOut() == 1 {
				if method.Type.Out(0).String() == "error" {
					// Storage for arguments and output types
					var args []string
					var output []string
					// Enumerate the input parameters,
					// 0 is the class its in - SKIP
					// 1 is http stuff - SKIP
					// 2 is the input parameter structure - Start here
					// 3 is the out structure
					for j := 2; j < method.Type.NumIn(); j++ {
						// Make sure that the input parameter is a structure
						if method.Type.In(j).Elem().Kind() == reflect.Struct {
							// Get the actual structure
							s := method.Type.In(j).Elem()
							// Enumerate through the parameter fields
							for k := 0; k < s.NumField(); k++ {
								// Try and get the json tag if it exists in the field
								// if not, default to the field name
								fieldName := s.Field(k).Tag.Get("json")
								if fieldName == "" {
									fieldName = s.Field(k).Name
								}
								// 3 is the output parameter which is a pointer, send it off to output slice
								if j == 3 {
									output = append(output, fmt.Sprintf("%s[%s]", fieldName, s.Field(k).Type.String()))
								} else {
									// Send everything else into the input slice
									args = append(args, fmt.Sprintf("%s[%s]", fieldName, s.Field(k).Type.String()))
								}
							}
						}
					}
					// If we have no arguments, make a `void` filler
					if len(args) == 0 {
						args = append(args, "void")
					}
					if len(output) == 0 {
						output = append(output, "void")
					}
					body += fmt.Sprintf("|FujisanRpc.%s|`%s`|`%s`|\n", method.Name, strings.Join(args, ", "), strings.Join(output, ", "))
				}
			}
		}
	}
	return string(markdown.ToHTML([]byte(body), nil, renderer)), nil
}

// Start Arguments

type MusicKitArgs struct {
	Endpoint string `json:"endpoint"`
	Body     string `json:"body"`
	Method   string `json:"method"`
}

type CallbackArgs struct {
	Url string `json:"url"`
}

type InfoType struct {
	Info interface{} `json:"info"`
}

type SuccessType struct {
	Success bool `json:"success"`
}

type ActiveType struct {
	Active bool `json:"active"`
}

type IsPlayingType struct {
	IsPlaying bool `json:"isPlaying"`
}

// End arguments

// Start RPC Methods

func (f *FujisanRpc) HandleCallbackUrl(r *http.Request, args *CallbackArgs, result *SuccessType) error {
	if result != nil {
		*result = SuccessType{true}
	}
	go FujisanObject.HandleCallback(args.Url)
	return nil
}

func (f *FujisanRpc) Active(r *http.Request, args *interface{}, result *ActiveType) error {
	*result = ActiveType{true}
	return nil
}

func (f *FujisanRpc) GetCurrentPlayingSong(r *http.Request, args *interface{}, result *InfoType) error {
	*result = InfoType{f.ExecuteAndReceiveJS("MusicKit.getInstance().nowPlayingItem.attributes")}
	return nil
}

func (f *FujisanRpc) IsPlaying(r *http.Request, args *interface{}, result *IsPlayingType) error {
	*result = IsPlayingType{f.ExecuteAndReceiveJS("MusicKit.getInstance().isPlaying").(bool)}
	return nil
}

func (f *FujisanRpc) PlayPause(r *http.Request, args *interface{}, result *RpcType) error {
	wruntime.WindowExecJS(FujisanObject.ctx, "if (MusicKit.getInstance().isPlaying) { MusicKit.getInstance().pause(); } else { MusicKit.getInstance().play(); }")
	return nil
}

func (f *FujisanRpc) Play(r *http.Request, args *interface{}, result *RpcType) error {
	wruntime.WindowExecJS(FujisanObject.ctx, "MusicKit.getInstance().play()")
	return nil
}

func (f *FujisanRpc) Pause(r *http.Request, args *interface{}, result *RpcType) error {
	wruntime.WindowExecJS(FujisanObject.ctx, "MusicKit.getInstance().pause()")
	return nil
}

func (f *FujisanRpc) Stop(r *http.Request, args *interface{}, result *interface{}) error {
	wruntime.WindowExecJS(FujisanObject.ctx, "MusicKit.getInstance().stop()")
	return nil
}

func (f *FujisanRpc) Next(r *http.Request, args *interface{}, result *interface{}) error {
	wruntime.WindowExecJS(FujisanObject.ctx, "MusicKit.getInstance().skipToNextItem()")
	return nil
}

func (f *FujisanRpc) Previous(r *http.Request, args *interface{}, result *interface{}) error {
	wruntime.WindowExecJS(FujisanObject.ctx, "MusicKit.getInstance().skipToPreviousItem()")
	return nil
}

type AlbumArgs struct {
	Url string `json:"url"`
}

type SeekToArgs struct {
	Second int `json:"second"`
}

func (f *FujisanRpc) SeekTo(r *http.Request, args *SeekToArgs, result *interface{}) error {
	if args == nil {
		return errors.New("must pass in seconds")
	}
	wruntime.WindowExecJS(FujisanObject.ctx, fmt.Sprintf("MusicKit.getInstance().skipToPreviousItem('%v')", args.Second))
	return nil
}

func (f *FujisanRpc) Hide(r *http.Request, args *interface{}, result *interface{}) error {
	wruntime.Hide(FujisanObject.ctx)
	return nil
}

func (f *FujisanRpc) Show(r *http.Request, args *interface{}, result *interface{}) error {
	wruntime.Show(FujisanObject.ctx)
	return nil
}

func (f *FujisanRpc) ExecuteAndReceiveJS(script string) interface{} {
	wruntime.WindowExecJS(FujisanObject.ctx, fmt.Sprintf(`go.main.Cider.HandleJSReturn(%v)`, script))
	select {
	case output := <-JavascriptChannel:
		JavascriptChannel = make(chan interface{})
		return output
	case <-time.After(1 * time.Second): // One second timeout if we don't receive anything in the channel
		return ""
	}
}

// End RPC methods
