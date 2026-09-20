package main

import (
	"runtime"
	"sort"
	"strings"
	"sync"

	"github.com/stenstromen/switchboard/config"
	"github.com/stenstromen/switchboard/ssh"
	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
)

const (
	mainWindowName     = "main"
	settingsWindowName = "settings"
)

// AppUI owns the main window, settings window, app menu, and system tray.
type AppUI struct {
	app     *application.App
	service *AppService

	tray *application.SystemTray

	mu             sync.Mutex
	refreshQueued  bool
	showAs         config.ShowAs
	sidebarVisible bool
	selectedID     string

	// Menu items updated when selection / connection state changes.
	menuEdit          *application.MenuItem
	menuDuplicate     *application.MenuItem
	menuConnect       *application.MenuItem
	menuDisconnect    *application.MenuItem
	menuInspector     *application.MenuItem
	menuNewTag        *application.MenuItem
	menuPin           *application.MenuItem
	menuSidebar       *application.MenuItem
	menuConnectAll    *application.MenuItem
	menuDisconnectAll *application.MenuItem
}

func NewAppUI(app *application.App, service *AppService) *AppUI {
	ui := &AppUI{app: app, service: service, showAs: config.ShowAsBoth, sidebarVisible: true}
	service.setUI(ui)
	return ui
}

func (ui *AppUI) Setup() {
	prefs, _ := ui.service.store.Preferences()
	ui.showAs = prefs.ShowAs.Normalize()

	ui.setupAppMenu()
	ui.createMainWindow()
	ui.setupTray()

	// Tray/dock APIs need the app running — ApplyShowAs uses InvokeAsync.
	ui.app.Event.OnApplicationEvent(events.Common.ApplicationStarted, func(*application.ApplicationEvent) {
		ui.applyShowAsNow(ui.showAs)
	})
}

func (ui *AppUI) setupAppMenu() {
	menu := ui.app.NewMenu()
	if runtime.GOOS == "darwin" {
		appMenu := menu.AddSubmenu("Switchboard")
		appMenu.AddRole(application.About)
		appMenu.AddSeparator()
		appMenu.Add("Settings…").SetAccelerator("CmdOrCtrl+,").OnClick(func(*application.Context) {
			ui.OpenSettings()
		})
		appMenu.AddSeparator()
		appMenu.AddRole(application.ServicesMenu)
		appMenu.AddSeparator()
		appMenu.AddRole(application.Hide)
		appMenu.AddRole(application.HideOthers)
		appMenu.AddRole(application.UnHide)
		appMenu.AddSeparator()
		appMenu.AddRole(application.Quit)
	}

	file := menu.AddSubmenu("File")
	file.Add("New Tunnel").SetAccelerator("CmdOrCtrl+N").OnClick(func(*application.Context) {
		ui.emitMenu("newTunnel")
	})
	ui.menuNewTag = file.Add("New Tag").SetAccelerator("Ctrl+CmdOrCtrl+N")
	ui.menuNewTag.OnClick(func(*application.Context) {
		ui.emitMenu("newTag")
	})
	file.AddSeparator()
	file.Add("Close").SetAccelerator("CmdOrCtrl+W").OnClick(func(*application.Context) {
		ui.hideMain()
	})
	file.AddSeparator()
	ui.menuInspector = file.Add("Show Inspector").SetAccelerator("CmdOrCtrl+I")
	ui.menuInspector.OnClick(func(*application.Context) {
		ui.emitMenu("showInspector")
	})

	menu.AddRole(application.EditMenu)

	view := menu.AddSubmenu("View")
	ui.menuSidebar = view.AddCheckbox("Show Sidebar", true).SetAccelerator("Ctrl+CmdOrCtrl+S")
	ui.menuSidebar.OnClick(func(*application.Context) {
		visible := ui.menuSidebar.Checked()
		ui.mu.Lock()
		ui.sidebarVisible = visible
		ui.mu.Unlock()
		ui.emitMenu("setSidebar", map[string]any{"visible": visible})
	})
	view.Add("Enter Full Screen").OnClick(func(*application.Context) {
		ui.toggleFullscreen()
	})

	tunnel := menu.AddSubmenu("Tunnel")
	ui.menuPin = tunnel.AddCheckbox("Pin", false)
	ui.menuPin.OnClick(func(*application.Context) {
		ui.emitMenu("setPinned", map[string]any{"pinned": ui.menuPin.Checked()})
	})
	ui.menuEdit = tunnel.Add("Edit Tunnel").SetAccelerator("Shift+CmdOrCtrl+,")
	ui.menuEdit.OnClick(func(*application.Context) {
		ui.emitMenu("editTunnel")
	})
	ui.menuDuplicate = tunnel.Add("Duplicate Tunnel").SetAccelerator("CmdOrCtrl+D")
	ui.menuDuplicate.OnClick(func(*application.Context) {
		ui.emitMenu("duplicateTunnel")
	})
	tunnel.AddSeparator()
	ui.menuConnect = tunnel.Add("Connect").SetAccelerator("CmdOrCtrl+K")
	ui.menuConnect.OnClick(func(*application.Context) {
		ui.emitMenu("connect")
	})
	ui.menuDisconnect = tunnel.Add("Disconnect").SetAccelerator("CmdOrCtrl+.")
	ui.menuDisconnect.OnClick(func(*application.Context) {
		ui.emitMenu("disconnect")
	})
	tunnel.AddSeparator()
	ui.menuConnectAll = tunnel.Add("Connect All Tunnels").SetAccelerator("Option+CmdOrCtrl+K")
	ui.menuConnectAll.OnClick(func(*application.Context) {
		_ = ui.service.ConnectAll()
	})
	ui.menuDisconnectAll = tunnel.Add("Disconnect All Tunnels").SetAccelerator("Option+CmdOrCtrl+.")
	ui.menuDisconnectAll.OnClick(func(*application.Context) {
		_ = ui.service.DisconnectAll()
	})

	menu.AddRole(application.WindowMenu)
	menu.AddRole(application.HelpMenu)
	ui.app.Menu.Set(menu)
	ui.refreshMenuState()
}

func (ui *AppUI) emitMenu(command string, extra ...map[string]any) {
	payload := map[string]any{"command": command}
	for _, m := range extra {
		for k, v := range m {
			payload[k] = v
		}
	}
	if app := application.Get(); app != nil {
		app.Event.Emit("menu:command", payload)
	}
}

func (ui *AppUI) hideMain() {
	if win, ok := ui.app.Window.GetByName(mainWindowName); ok {
		win.Hide()
	}
}

func (ui *AppUI) toggleFullscreen() {
	if win, ok := ui.app.Window.GetByName(mainWindowName); ok {
		win.ToggleFullscreen()
	}
}

// SetSelection updates which tunnel is selected so menu items enable correctly.
func (ui *AppUI) SetSelection(id string) {
	ui.mu.Lock()
	ui.selectedID = id
	ui.mu.Unlock()
	ui.RefreshMenuState()
}

// RefreshMenuState re-evaluates enabled/disabled app menu items.
func (ui *AppUI) RefreshMenuState() {
	ui.refreshMenuState()
}

func (ui *AppUI) refreshMenuState() {
	ui.mu.Lock()
	id := ui.selectedID
	ui.mu.Unlock()

	hasSel := id != ""
	connected := false
	pinned := false
	anyConnected := false
	anyDisconnected := false
	if tunnels, err := ui.service.ListTunnels(); err == nil {
		for _, t := range tunnels {
			live := t.Status.Status == ssh.StatusConnected || t.Status.Status == ssh.StatusConnecting
			if live {
				anyConnected = true
			} else {
				anyDisconnected = true
			}
			if t.Profile.ID == id {
				connected = live
				pinned = t.Profile.Pinned
			}
		}
	}

	set := func(item *application.MenuItem, enabled bool) {
		if item != nil {
			item.SetEnabled(enabled)
		}
	}
	set(ui.menuEdit, hasSel)
	set(ui.menuDuplicate, hasSel)
	set(ui.menuInspector, hasSel)
	set(ui.menuNewTag, hasSel)
	set(ui.menuPin, hasSel)
	if ui.menuPin != nil {
		ui.menuPin.SetChecked(pinned)
	}
	set(ui.menuConnect, hasSel && !connected)
	set(ui.menuDisconnect, hasSel && connected)
	set(ui.menuConnectAll, anyDisconnected)
	set(ui.menuDisconnectAll, anyConnected)
}

func (ui *AppUI) createMainWindow() {
	win := ui.app.Window.NewWithOptions(application.WebviewWindowOptions{
		Name:      mainWindowName,
		Title:     "Switchboard",
		Width:     1100,
		Height:    720,
		MinWidth:  900,
		MinHeight: 560,
		Mac: application.MacWindow{
			InvisibleTitleBarHeight: 52,
			Backdrop:                application.MacBackdropTranslucent,
			TitleBar:                application.MacTitleBarHiddenInset,
		},
		BackgroundColour: application.NewRGB(28, 26, 34),
		URL:              "/",
	})

	// Hide instead of quit when a tray (or dock) can bring the app back.
	win.RegisterHook(events.Common.WindowClosing, func(e *application.WindowEvent) {
		win.Hide()
		e.Cancel()
	})
}

func (ui *AppUI) ensureSettingsWindow() application.Window {
	if win, ok := ui.app.Window.GetByName(settingsWindowName); ok {
		return win
	}
	return ui.app.Window.NewWithOptions(application.WebviewWindowOptions{
		Name:      settingsWindowName,
		Title:     "Settings",
		Width:     640,
		Height:    320,
		MinWidth:  560,
		MinHeight: 260,
		Mac: application.MacWindow{
			InvisibleTitleBarHeight: 28,
			Backdrop:                application.MacBackdropTranslucent,
			TitleBar:                application.MacTitleBarHiddenInset,
		},
		BackgroundColour: application.NewRGB(30, 28, 36),
		URL:              "/?page=settings",
	})
}

func (ui *AppUI) setupTray() {
	tray := ui.app.SystemTray.New()
	ui.tray = tray
	// Template glyphs (black + alpha): system tints them; we swap bright/dim opacity.
	ui.tray.SetTemplateIcon(trayInactiveIcon)
	ui.refreshTrayMenu()
}

// applyTrayIcon switches the menu-bar glyph: bright when any tunnel is live.
func (ui *AppUI) applyTrayIcon(active bool) {
	if ui.tray == nil {
		return
	}
	if active {
		ui.tray.SetTemplateIcon(trayActiveIcon)
	} else {
		ui.tray.SetTemplateIcon(trayInactiveIcon)
	}
}

// ApplyShowAs updates Dock visibility and menu-bar tray visibility.
func (ui *AppUI) ApplyShowAs(showAs config.ShowAs) {
	showAs = showAs.Normalize()
	ui.mu.Lock()
	ui.showAs = showAs
	ui.mu.Unlock()

	application.InvokeAsync(func() {
		ui.applyShowAsNow(showAs)
	})
}

func (ui *AppUI) applyShowAsNow(showAs config.ShowAs) {
	showDock := showAs == config.ShowAsDock || showAs == config.ShowAsBoth
	showTray := showAs == config.ShowAsMenuBar || showAs == config.ShowAsBoth

	setDockVisible(showDock)

	if ui.tray == nil {
		return
	}
	if showTray {
		ui.refreshTrayMenu()
		ui.tray.Show()
	} else {
		ui.tray.Hide()
	}
}

func (ui *AppUI) OpenMain() {
	win, ok := ui.app.Window.GetByName(mainWindowName)
	if !ok {
		return
	}
	win.Show()
	win.Focus()
}

func (ui *AppUI) OpenSettings() {
	win := ui.ensureSettingsWindow()
	win.Show()
	win.Focus()
	win.Center()
}

// RefreshTray schedules a tray menu rebuild (coalesced).
func (ui *AppUI) RefreshTray() {
	ui.mu.Lock()
	showAs := ui.showAs
	if ui.refreshQueued {
		ui.mu.Unlock()
		return
	}
	ui.refreshQueued = true
	ui.mu.Unlock()

	application.InvokeAsync(func() {
		ui.mu.Lock()
		ui.refreshQueued = false
		ui.mu.Unlock()
		if showAs == config.ShowAsDock {
			return
		}
		ui.refreshTrayMenu()
	})
}

func (ui *AppUI) refreshTrayMenu() {
	if ui.tray == nil {
		return
	}

	menu := ui.app.NewMenu()

	tunnels, _ := ui.service.ListTunnels()
	anyConnected := false
	anyDisconnected := false
	for _, t := range tunnels {
		st := t.Status.Status
		if st == ssh.StatusConnected || st == ssh.StatusConnecting {
			anyConnected = true
		} else {
			anyDisconnected = true
		}
	}
	ui.applyTrayIcon(anyConnected)

	menu.Add("Connect All").
		SetAccelerator("Option+CmdOrCtrl+K").
		SetEnabled(anyDisconnected).
		OnClick(func(*application.Context) {
			_ = ui.service.ConnectAll()
		})
	menu.Add("Disconnect All").
		SetAccelerator("Option+CmdOrCtrl+.").
		SetEnabled(anyConnected).
		OnClick(func(*application.Context) {
			_ = ui.service.DisconnectAll()
		})

	menu.AddSeparator()
	menu.Add("Open Main Window").OnClick(func(*application.Context) {
		ui.OpenMain()
	})
	menu.Add("Settings…").OnClick(func(*application.Context) {
		ui.OpenSettings()
	})

	menu.AddSeparator()
	ui.appendTrayTunnelSection(menu, tunnels)

	menu.AddSeparator()
	menu.Add("Quit Switchboard").SetAccelerator("CmdOrCtrl+Q").OnClick(func(*application.Context) {
		ui.app.Quit()
	})

	ui.tray.SetMenu(menu)
}

// appendTrayTunnelSection adds either a flat tunnel list (no tags) or one
// submenu per tag with Connect/Disconnect All, matching Core Tunnel's tray UX.
func (ui *AppUI) appendTrayTunnelSection(menu *application.Menu, tunnels []TunnelView) {
	if len(tunnels) == 0 {
		menu.Add("Tunnels").SetEnabled(false)
		menu.Add("No Tunnels").SetEnabled(false)
		return
	}

	tags, byTag, untagged := groupTunnelsByTag(tunnels)
	if len(tags) == 0 {
		menu.Add("Tunnels").SetEnabled(false)
		ui.appendTrayTunnelItems(menu, tunnels)
		return
	}

	for _, tag := range tags {
		group := byTag[tag]
		sub := menu.AddSubmenu(tag)
		ui.appendTrayTagControls(sub, tag, group)
		ui.appendTrayTunnelItems(sub, group)
	}
	if len(untagged) > 0 {
		sub := menu.AddSubmenu("Untagged")
		ui.appendTrayTagControls(sub, "", untagged)
		ui.appendTrayTunnelItems(sub, untagged)
	}
}

func (ui *AppUI) appendTrayTagControls(menu *application.Menu, tag string, group []TunnelView) {
	anyConnected, anyDisconnected := tunnelLiveFlags(group)
	menu.Add("Connect All").
		SetEnabled(anyDisconnected).
		OnClick(func(*application.Context) {
			_ = ui.service.ConnectByTag(tag)
		})
	menu.Add("Disconnect All").
		SetEnabled(anyConnected).
		OnClick(func(*application.Context) {
			_ = ui.service.DisconnectByTag(tag)
		})
	menu.AddSeparator()
}

func (ui *AppUI) appendTrayTunnelItems(menu *application.Menu, tunnels []TunnelView) {
	for _, t := range tunnels {
		id := t.Profile.ID
		name := trayTunnelLabel(t.Profile)
		connected := t.Status.Status == ssh.StatusConnected || t.Status.Status == ssh.StatusConnecting
		item := menu.AddCheckbox(name, connected)
		item.OnClick(func(ctx *application.Context) {
			if ctx.ClickedMenuItem().Checked() {
				_ = ui.service.Connect(id)
			} else {
				_ = ui.service.Disconnect(id)
			}
		})
	}
}

func trayTunnelLabel(p config.Profile) string {
	if p.Name != "" {
		return p.Name
	}
	if p.HostName != "" {
		return p.HostName
	}
	return "Unnamed"
}

func tunnelLiveFlags(tunnels []TunnelView) (anyConnected, anyDisconnected bool) {
	for _, t := range tunnels {
		live := t.Status.Status == ssh.StatusConnected || t.Status.Status == ssh.StatusConnecting
		if live {
			anyConnected = true
		} else {
			anyDisconnected = true
		}
	}
	return anyConnected, anyDisconnected
}

func groupTunnelsByTag(tunnels []TunnelView) (tags []string, byTag map[string][]TunnelView, untagged []TunnelView) {
	byTag = make(map[string][]TunnelView)
	seen := make(map[string]bool)
	for _, t := range tunnels {
		if len(t.Profile.Tags) == 0 {
			untagged = append(untagged, t)
			continue
		}
		for _, tag := range t.Profile.Tags {
			tag = strings.TrimSpace(tag)
			if tag == "" {
				continue
			}
			if !seen[tag] {
				seen[tag] = true
				tags = append(tags, tag)
			}
			byTag[tag] = append(byTag[tag], t)
		}
	}
	sort.Strings(tags)
	return tags, byTag, untagged
}
