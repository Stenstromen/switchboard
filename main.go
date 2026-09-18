package main

import (
	"embed"
	"log"
	"os"

	"github.com/stenstromen/switchboard/config"
	"github.com/stenstromen/switchboard/ssh"
	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/services/notifications"
)

//go:embed all:frontend/dist
var assets embed.FS

//go:embed assets/appicon.png
var appIcon []byte

//go:embed assets/trayInactive.png
var trayInactiveIcon []byte

//go:embed assets/trayActive.png
var trayActiveIcon []byte

func init() {
	application.RegisterEvent[StatusEvent]("tunnel:status")
	application.RegisterEvent[PasswordPrompt]("tunnel:password")
	application.RegisterEvent[ConfigChangedEvent]("config:changed")
}

func main() {
	if ssh.IsAskpass() {
		os.Exit(ssh.RunAskpass())
	}

	store, err := config.Open("")
	if err != nil {
		log.Fatal(err)
	}

	prefs, _ := store.Preferences()
	activation := application.ActivationPolicyRegular
	if prefs.ShowAs == config.ShowAsMenuBar {
		activation = application.ActivationPolicyAccessory
	}

	notifier := notifications.New()
	service := NewAppService(store, notifier)
	defer service.Close()

	app := application.New(application.Options{
		Name:        "Switchboard",
		Description: "SSH tunnel manager around OpenSSH",
		Icon:        appIcon,
		Services: []application.Service{
			application.NewService(service),
			application.NewService(notifier),
		},
		Assets: application.AssetOptions{
			Handler: application.AssetFileServerFS(assets),
		},
		Mac: application.MacOptions{
			ActivationPolicy: activation,
			// Tray / Dock keep the process alive when all windows are hidden.
			ApplicationShouldTerminateAfterLastWindowClosed: false,
		},
	})

	ui := NewAppUI(app, service)
	ui.Setup()

	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}
