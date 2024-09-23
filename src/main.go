package main

import (
	"encoding/json"
	"io"
	"log"
	"os"
	"path/filepath"

	"github.com/ciderapp/wails/v2"
	"github.com/ciderapp/wails/v2/pkg/options"
	"github.com/ciderapp/wails/v2/pkg/options/assetserver"
	"github.com/ciderapp/wails/v2/pkg/options/mac"
	"github.com/ciderapp/wails/v2/pkg/options/windows"
	yomikaki "github.com/freehelpdesk/yomikaki"
	"gopkg.in/natefinch/lumberjack.v2"
)

var (
	BackgroundColor = options.RGBA{R: 0, G: 0, B: 0, A: 0}
	Writer          io.Writer
)

func main() {
	var config map[string]interface{}

	if err := json.Unmarshal([]byte(FujisanIOObject.ReadFile("spa-config.json")), &config); err != nil {
		log.Println("Failed to cast json to map")
	}

	// Setup logger
	Writer = io.MultiWriter(&lumberjack.Logger{
		Filename:   filepath.Join(FujisanIOObject.GetConfigPath(), "fujisan.log"),
		MaxSize:    5,
		MaxBackups: 3,
		MaxAge:     28,
		Compress:   false,
	}, os.Stdout)

	log.SetOutput(Writer)

	gpuIsDisabled := false

	hardwareAccelInterface, _ := yomikaki.DirectRead("visual.hardwareAcceleration", config)
	if hardwareAccelInterface == nil {
		hardwareAccelInterface = "webgpu"
	}

	switch hardwareAccelInterface.(string) {
	case "disabled":
		gpuIsDisabled = true
	default:
		gpuIsDisabled = false
	}

	os.Setenv("WEBVIEW2_ADDITIONAL_BROWSER_ARGUMENTS", "--disable-web-security --js-flags=--expose-gc,--max-old-space-size=350 --max-old-space-size=350 --autoplay-policy=no-user-gesture-required --force_high_performance_gpu --enable-featuresEnableDrDc,CanvasOopRasterization,BackForwardCache:TimeToLiveInBackForwardCacheInSeconds/300/should_ignore_blocklists/true/enable_same_site/true,ThrottleDisplayNoneAndVisibilityHiddenCrossOriginIframes,UseSkiaRenderer,WebAssemblyLazyCompilation,EdgeOverlayScrollbarsWinStyle --enable-hardware-overlays=single-fullscreen,single-on-top,underlay --ignore-gpu-blocklist --enable-zero-copy --enable-gpu-rasterization --enable-unsafe-webgpu --num-raster-threads=4")

	// Setup cider
	go FujisanObject.Run()

	// Do window management
	window := &windows.Options{
		WebviewIsTransparent: true,
		WindowIsTranslucent:  true,
		Theme:                windows.Dark,
		WebviewUserDataPath:  FujisanIOObject.GetConfigPath(),
		WebviewGpuIsDisabled: gpuIsDisabled,
	}

	themeInterface, _ := yomikaki.DirectRead("visual.theme", config)
	if themeInterface == nil {
		themeInterface = ""
	}

	if FujisanObject.GetOS() == "windows" {
		log.Println("'" + themeInterface.(string) + "'")

		switch themeInterface.(string) {
		case "opaque":
			log.Println("Setting theme to Opaque")
			window.BackdropType = windows.None
			window.WebviewIsTransparent = false
			window.WindowIsTranslucent = false
		case "acrylic":
			log.Println("Setting theme to Acrylic")
			window.BackdropType = windows.Acrylic
		case "mica":
			log.Println("Setting theme to Mica")
			window.BackdropType = windows.Mica
		case "tabbed":
			fallthrough
		default:
			log.Println("Defaulting theme to Tabbed")
			window.BackdropType = windows.Tabbed
		}

		if !FujisanObject.IsWindows11() && themeInterface.(string) != "acrylic" {
			log.Println("Disabling transparency below Windows 11 if not using acrylic")
			window.WebviewIsTransparent = false
			window.WindowIsTranslucent = false
			window.BackdropType = windows.None
		}
	}

	newHeaders.Set("User-Agent", "[PLACEHOLDER]")

	opts := &options.App{
		Title:     "Fujisan",
		Width:     1380,
		Height:    730,
		MinWidth:  690,
		MinHeight: 365,
		AssetServer: &assetserver.Options{
			Assets: FujisanAssets,
		},
		BackgroundColour: &BackgroundColor,
		OnStartup:        FujisanObject.startup,
		OnDomReady:       FujisanObject.OnDomReady,
		OnBeforeClose:    FujisanObject.shutdown,
		Frameless:        true,
		Bind: []interface{}{
			FujisanObject,
			FujisanIOObject,
			FujisanKasumiObject,
		},
		StartUrl:        "[PLACEHOLDER]",
		Headers:         newHeaders,
		CSSDragProperty: "-webkit-app-region",
		CSSDragValue:    "drag",
		Windows:         window,
		Mac: &mac.Options{
			Appearance:           mac.DefaultAppearance,
			WebviewIsTransparent: true,
			WindowIsTranslucent:  true,
		},
	}

	if FujisanObject.GetOS() == "darwin" {
		opts.Frameless = false
		opts.StartUrl = ""
	}
	if err := wails.Run(opts); err != nil {
		log.Println("Error:", err.Error())
	}
}
