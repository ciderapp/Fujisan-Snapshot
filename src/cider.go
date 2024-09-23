package main

import (
	"bytes"
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"time"

	yomikaki "github.com/freehelpdesk/yomikaki"

	"github.com/ciderapp/kasumi"
	"github.com/ciderapp/lastfm-go/lastfm"
	"github.com/ciderapp/rich-go/client"
	wruntime "github.com/ciderapp/wails/v2/pkg/runtime"
	"github.com/gorilla/mux"
	"github.com/gorilla/rpc"
	gjson "github.com/gorilla/rpc/json"
)

// Objects
var (
	FujisanRpcObject        = new(FujisanRpc)
	FujisanObject           = CreateCider()
	FujisanIOObject         = NewIO()
	FujisanKasumiObject     = kasumi.New(&kasumi.Config{ApplicationName: "fujisan"})
	FujisanDiscordRpcObject = client.New()

	//go:embed all:frontend/dist
	FujisanAssets embed.FS

	FujisanDOMAlreadyRan bool
) // End Objects

// Cider Main application structure which contains methods to pass through to the front end
type Cider struct {
	ctx              context.Context
	Activity         client.Activity
	LastFm           *lastfm.Api
	discordRPCStatus bool
}

// CreateCider creates a new Cider application struct and returns it as a `*Cider`
func CreateCider() *Cider {
	return &Cider{}
}

// SendRPC is an internal function to communicate with the RPC by sending a http request with the method, and parameters it needs to call
func (c *Cider) SendRPC(address string, method string, args interface{}) (interface{}, error) {
	client := new(http.Client)
	client.Timeout = 500 * time.Millisecond
	requestBody, err := gjson.EncodeClientRequest(method, args)
	if err != nil {
		return nil, err
	}
	log.Println(string(requestBody))
	request, err := http.NewRequest("POST", address, bytes.NewBuffer(requestBody))
	if err != nil {
		return nil, err
	}
	request.Header.Set("Content-Type", "application/json")
	response, err := client.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	var body interface{}
	if err = gjson.DecodeClientResponse(response.Body, &body); err != nil {
		return nil, err
	}
	log.Println(body)
	return body, nil
}

// startup is called when the app starts. The context is saved
// so we can call the runtime methods
func (c *Cider) startup(ctx context.Context) {
	c.ctx = ctx
	log.Println("Starting up Fujisan Core")
	if err := c.RegisterCallbackUrl(); err != nil {
		log.Println("Failed to register URL callback scheme err:", err.Error())
	}

	var config map[string]interface{}
	if err := json.Unmarshal([]byte(FujisanIOObject.ReadFile("spa-config.json")), &config); err != nil {
		log.Println("Unable to cast json to struct")
	}

	var size []int
	read, err := yomikaki.DirectRead("visual.windowSize", config)
	if err != nil {
		log.Println(err)
	}

	if read != nil {
		size, _ = c.convertInterfaceToIntArray(read)
	}

	wruntime.WindowSetSize(FujisanObject.ctx, size[0], size[1])

	// Assure lengths are > 1
	//if len(config.Visual.WindowPosition) > 1 {
	//	// Broken at the moment, not sure why.
	//	// wruntime.WindowSetPosition(FujisanObject.ctx, config.Visual.WindowPosition[0], config.Visual.WindowPosition[1])
	//}

	// Starting two net services, one a unix socket, one a TCP service.
	_, err = net.Dial("tcp", "localhost:10782")
	if err != nil {
		// Could not connect to RPC, which means we are the host now
		// Register FujisanObject RPC
		log.Println("Starting Fujisan RPC")
		rpcServer := rpc.NewServer()
		rpcServer.RegisterCodec(gjson.NewCodec(), "application/json")
		rpcServer.RegisterCodec(gjson.NewCodec(), "application/json;charset=UTF-8")

		rpcServer.RegisterService(FujisanRpcObject, "FujisanRpc")
		router := mux.NewRouter()
		router.HandleFunc("/", func(writer http.ResponseWriter, request *http.Request) {
			page, err := FujisanRpcObject.generateDocsPage()
			if err != nil {
				writer.WriteHeader(http.StatusInternalServerError)
				return
			}
			writer.WriteHeader(http.StatusOK)
			_, _ = fmt.Fprintf(writer, page)
		}).Methods("GET").Schemes("http")

		router.Handle("/rpc", rpcServer)

		go func() {
			if err := http.ListenAndServe(":10782", router); err != nil {
				log.Println("Unable to start Fujisan RPC")
				wruntime.MessageDialog(FujisanObject.ctx, wruntime.MessageDialogOptions{
					Type:    wruntime.WarningDialog,
					Title:   "Error",
					Message: "Unable to start RPC on port 10782, connect and protocol operations will no longer work until this port has been freed",
				})
			}
		}()

		if len(os.Args) > 1 {
			c.HandleCallback(os.Args[1])
		}
	} else if len(os.Args) > 1 {
		c.SendRPC("http://localhost:10782/rpc", "FujisanRpc.HandleCallbackUrl", CallbackArgs{Url: os.Args[1]})
		os.Exit(0)
	} else {
		os.Exit(0)
	}

	// Register some events so we can not die later on
	wruntime.EventsOn(FujisanObject.ctx, "minimize", c.saveWindowInformation)

	log.Println("Startup complete")
}

func (c *Cider) saveWindowInformation(optionalData ...interface{}) {
	var config map[string]interface{}
	if err := json.Unmarshal([]byte(FujisanIOObject.ReadFile("spa-config.json")), &config); err != nil {
		log.Println("Unable to cast json to struct:", err)
		return
	}

	// TODO: WindowGetPosition only returns the position relative the the monitor it was opened on.
	// which means its messed up on multi monitor setups.
	//// Save window position and size
	//config.Visual.WindowPosition = make([]int, 2)
	//config.Visual.WindowPosition[0], config.Visual.WindowPosition[1] = wruntime.WindowGetPosition(FujisanObject.ctx)

	// Is the window is minimized, do not save.

	sz := make([]int, 2)
	sz[0], sz[1] = wruntime.WindowGetSize(FujisanObject.ctx)
	config = yomikaki.DirectWrite("visual.windowSize", config, sz)

	b, err := json.MarshalIndent(config, "", "\t")
	if err != nil {
		log.Println(err)
		return
	}
	FujisanIOObject.WriteFile("spa-config.json", string(b))
}

func (c *Cider) shutdown(ctx context.Context) bool {
	// Only save the window information if the current window is not minimized
	// in this case, we will use the event to capture the window state
	if !wruntime.WindowIsMinimised(FujisanObject.ctx) {
		c.saveWindowInformation()
	}
	return false
}

// HandleCallback sends over the URL without the scheme over to the frontend at `CiderApp.handleProtocolURL`
func (c *Cider) HandleCallback(url string) {
	log.Println("Got URL:", url)
	split := strings.Split(url, "://")
	if len(split) == 1 {
		log.Println("Failed to parse link")
		return
	}

	log.Println(strings.ToLower(split[1]))

	if strings.Contains(strings.ToLower(split[1]), "show") {
		wruntime.Show(FujisanObject.ctx)
	} else {
		wruntime.WindowExecJS(FujisanObject.ctx, fmt.Sprintf("CiderApp.handleProtocolURL('%s')", split[1]))
	}
}

func (c *Cider) GetVersion() string {
	return Version
}

// OpenDevToolsWindow Expose this to JS just in case
func (c *Cider) OpenDevToolsWindow() {
	wruntime.OpenDevToolsWindow(c.ctx)
}

// Run automatically started
func (c *Cider) Run() {
}

func (c *Cider) OnDomReady(ctx context.Context) {
	if !FujisanDOMAlreadyRan {
		FujisanDOMAlreadyRan = true
		log.Println("Dom is ready")
		// Load plugins
		plugins := NewPluginLoader(filepath.Join(FujisanIOObject.GetConfigPath(), "plugins"))
		plugins.LoadPlugins()
		log.Println("Finished DOM tasks")
		//plugins.PluginWatcher()
	}
}

// StartRichPresence initializes discord rich presence with our app id
func (c *Cider) StartRichPresence() {
	// Start rich presence

	var config map[string]interface{}
	if err := json.Unmarshal([]byte(FujisanIOObject.ReadFile("spa-config.json")), &config); err != nil {
		log.Println("Unable to cast json to struct:", err)
		return
	}

	clientId := "1032800329332445255"

	client := "Cider"
	read, err := yomikaki.DirectRead("connectivity.discord.client", config)
	if err != nil {
		log.Println(err)
	}

	if read != nil {
		client = read.(string)
	}

	switch client {
	case "AppleMusic":
		clientId = "886578863147192350"
	case "Cider-2":
		clientId = "1020414178047041627"
	case "Cider":
		fallthrough
	default:
		clientId = "911790844204437504"
	}

	// Try to login if we don't have a client
	if err := FujisanDiscordRpcObject.Login(clientId); err == nil {
		log.Println("Started rich presence")
		c.discordRPCStatus = true
	} else {
		log.Println("Failed to start rich presence")
	}
}

// IdlePresence Sets the client status to idle in discord.
func (c *Cider) IdlePresence() {
	c.StartRichPresence()
	if c.discordRPCStatus {
		now := time.Now()
		log.Println("Discord RPC going idle")
		if err := FujisanDiscordRpcObject.SetActivity(client.Activity{
			Details:    "Browsing Cider",
			LargeImage: "",
			Timestamps: &client.Timestamps{
				Start: &now,
			},
		}); err != nil {
			// Error here.
			log.Println(err.Error())
		}
	}
}

func (c *Cider) emptyIfNil(value interface{}) interface{} {
	if value == nil {
		// Get the type of the value
		valueType := reflect.TypeOf(value)
		// Create a new instance of the type with the zero value
		emptyValue := reflect.New(valueType).Elem().Interface()
		// Return the empty value
		return emptyValue
	}
	return value
}

func (c *Cider) convertInterfaceToIntArray(input interface{}) ([]int, error) {
	if arr, ok := input.([]interface{}); ok {
		myInts := make([]int, len(arr))
		for i, v := range arr {
			if val, ok := v.(float64); ok {
				log.Println(val)
				myInts[i] = int(val)
			}
		}
		// print the integer array
		return myInts, nil
	}
	return []int{}, fmt.Errorf("not an []interface{}")
}

func (c *Cider) formatDiscordStatus(text string, attributes Attributes) string {
	text = strings.ReplaceAll(text, "{artist}", attributes.ArtistName)
	text = strings.ReplaceAll(text, "{name}", attributes.Name)
	text = strings.ReplaceAll(text, "{album}", attributes.AlbumName)
	return text
}

// UpdatePresence updates the discord rich presence based on the given attributes
func (c *Cider) UpdatePresence(attributes Attributes) {
	var config map[string]interface{}
	if err := json.Unmarshal([]byte(FujisanIOObject.ReadFile("spa-config.json")), &config); err != nil {
		log.Println("Unable to cast json to struct:", err)
		return
	}

	c.StartRichPresence()
	if c.discordRPCStatus {
		now := time.Now() // Start time doesn't really matter because latency comes into play and the end timestamp doesn't change.
		end := time.UnixMilli(attributes.EndTime)

		detailsFormatInterface, _ := yomikaki.DirectRead("connectivity.discord.detailsFormat", config)
		stateFormatInterface, _ := yomikaki.DirectRead("connectivity.discord.stateFormat", config)
		hideTimstampInterface, _ := yomikaki.DirectRead("connectivity.discord.hideTimestamp", config)
		if hideTimstampInterface == nil {
			hideTimstampInterface = false
		}
		hideButtonsInterface, _ := yomikaki.DirectRead("connectivity.discord.hideButtons", config)
		if hideButtonsInterface == nil {
			hideButtonsInterface = false
		}

		c.Activity = client.Activity{
			Details:    c.formatDiscordStatus(c.emptyIfNil(detailsFormatInterface).(string), attributes),
			State:      c.formatDiscordStatus(c.emptyIfNil(stateFormatInterface).(string), attributes),
			LargeImage: c.SetImageResolution(1024, 1024, attributes.Artwork.URL),
			LargeText:  attributes.AlbumName,
		}

		if !hideTimstampInterface.(bool) {
			// TODO: Sanitize timestamps
			if !time.Now().After(end) {
				c.Activity.Timestamps = &client.Timestamps{
					Start: &now,
					End:   &end,
				}
			}
		}

		if !hideTimstampInterface.(bool) {
			c.Activity.Buttons = []*client.Button{{
				Label: "Play on Cider",
				Url:   "",
			}}
		}

		if err := FujisanDiscordRpcObject.SetActivity(c.Activity); err != nil {
			log.Println(err.Error())
		}
	}
	// Need to check for duplicates; MusicKit loves to fire this event twice, standby to see if we cant prevent it from sending it over javascript.
}

// UpdatePresenceOptions allows us to update buttons, and switch on and off Rich Presence while its running
func (c *Cider) UpdatePresenceOptions(options RpcOptions) {
	c.StartRichPresence()
	if options.Enabled {
		if !c.discordRPCStatus {
			c.StartRichPresence()
			c.discordRPCStatus = true
		}
	} else {
		FujisanDiscordRpcObject.Logout()
		c.discordRPCStatus = false
	}

	if c.discordRPCStatus {
		if len(options.Buttons) > 0 {
			c.Activity.Buttons = []*client.Button{}
			for _, button := range options.Buttons {
				c.Activity.Buttons = append(c.Activity.Buttons, &client.Button{Label: button.Label, Url: button.Url})
			}
		}
		if options.Paused {
			c.Activity.Timestamps = nil
			c.Activity.Details = fmt.Sprintf("%s - Paused", c.Activity.Details)
		}
		if err := FujisanDiscordRpcObject.SetActivity(c.Activity); err != nil {
			log.Println(err.Error())
		}
	}
	// Need to check for duplicates; MusicKit loves to fire this event twice, standby to see if we cant prevent it from sending it over javascript.
}

// TokenExists checks to see if the LastFM token has been set
func (c *Cider) TokenExists() bool {
	if _, err := c.LastFm.GetToken(); err != nil {
		return false
	}
	return true
}

// ScrobbleSong srobbles a song via song attributes passed from the frontend
func (c *Cider) ScrobbleSong(attributes Attributes) {
	if c.LastFm != nil {
		if !c.TokenExists() {
			log.Println("Failed to get user token, failed to login.")
			return
		}

		p := lastfm.P{
			"artist":    attributes.ArtistName,
			"album":     attributes.AlbumName,
			"track":     attributes.Name,
			"timestamp": time.Now().Unix(),
		}

		if _, err := c.LastFm.Track.UpdateNowPlaying(p); err != nil {
			log.Println("Failed to update Now Playing.", err)
			return
		}

		if _, err := c.LastFm.Track.Scrobble(p); err != nil {
			log.Println("Failed to scrobble song.", err)
			return
		}
	}
}

// QuerySong gets a song based on artist and song name
func (c *Cider) QuerySong(attributes Attributes) string {
	if c.LastFm != nil {
		if !c.TokenExists() {
			log.Println("Failed to get user token, failed to login.")
			return ""
		}

		p := lastfm.P{
			"artist": attributes.ArtistName,
			"track":  attributes.Name,
		}
		search, err := c.LastFm.Track.Search(p)
		if err != nil {
			return ""
		}
		jsonSearch, _ := json.Marshal(search)
		return string(jsonSearch)
	}
	return ""
}

// InitLastFM sets all the necessary configuration options for LastFM to work
func (c *Cider) InitLastFM(key string, secret string, account string, password string) {
	if len(account) == 0 {
		log.Println("Not logging into LastFM. Account is not setup.")
		return
	}
	c.LastFm = lastfm.New(key, secret)
	if err := c.LastFm.Login(account, password); err != nil {
		log.Println("Failed to login to LastFM.", err)
		return
	}
}

func (c *Cider) CastMedia() {

}

func (c *Cider) InitCast() {

}

func (c *Cider) CastSources() {

}

// GetOSBuild returns the windows build number, used to check if we can enable transparency
func (c *Cider) GetOSBuild() int {
	return int(GetVersion())
}

func (c *Cider) GetOS() string {
	return runtime.GOOS
}

// IsWindows11 returns if the current Windows version is above 22000, the start of Windows 11
func (c *Cider) IsWindows11() bool {
	return c.GetOS() == "windows" && c.GetOSBuild() > 22000
}

// SetImageResolution replaces {w}x{h} in the Url to the width and height specified
func (c *Cider) SetImageResolution(x int, y int, url string) string {
	return strings.ReplaceAll(url, "{w}x{h}", fmt.Sprintf("%vx%v", x, y))
}

// InDevMode returns if the app is running in development mode
func (c *Cider) InDevMode() bool {
	return os.Getenv("WAILS_RUNTIME_MODE") == "development"
}

// HandleJSReturn is called by the frontend and send its message to `JavascriptChannel`
func (c *Cider) HandleJSReturn(output interface{}) {
	// Send the input over a channel to the waiting method over in rpc.go ExecuteAndReceiveJS
	JavascriptChannel <- output
}
