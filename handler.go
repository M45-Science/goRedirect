package main

import (
	"fmt"
	"goRedirect/cwlog"
	"goRedirect/sclean"
	"html"
	"html/template"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	strip "github.com/grokify/html-strip-tags-go"
)

const (
	redirectPrefix = "/gosteam/"
	maxUrlLen      = 128
)

// Define a struct to hold template data
type TemplateData struct {
	Title    string
	AppName  string
	AppID    uint64
	SteamURL string
	Command  string
}

func httpsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		return
	}
	if !strings.HasPrefix(r.URL.Path, redirectPrefix) {
		fileServer.ServeHTTP(w, r)
		return
	}

	// Stop if URL is too long
	if len(r.URL.Path) > maxUrlLen {
		return
	}

	// Extract and clean URL parts
	input := html.UnescapeString(r.URL.Path)
	input = strip.StripTags(input)
	args := strings.SplitN(input, ".", 2)
	if len(args) != 2 {
		w.Write([]byte("Usage: https://go-game.net/gosteam/123456.command\nResult: steam://run/123456//command/"))
		cwlog.DoLog(true, "Invalid number of arguments: %v", input)
		return
	}

	// Parse application ID and command
	appinput, command := strings.TrimPrefix(args[0], redirectPrefix), args[1]
	appint, err := strconv.ParseUint(appinput, 10, 64)
	if err != nil {
		return
	}

	// Retrieve app name, or set as unknown if not found
	var appName string
	dbLock.RLock()
	appData := appList[appint]
	dbLock.RUnlock()

	if appData != nil {
		appName = sclean.StripControlAndSpecial(appData.Name)
	} else {
		if time.Since(lastUpdate) > updateInterval {
			dbLock.Lock()
			updateDatabase(true)
			appData = appList[appint]
			dbLock.Unlock()
		}
		if appData == nil {
			appName = "UNKNOWN"
		} else {
			appName = sclean.StripControlAndSpecial(appData.Name)
		}
	}

	// Prepare data for the template
	data := TemplateData{
		Title:    "GoSteam Redirect Service",
		AppName:  appName,
		AppID:    appint,
		SteamURL: fmt.Sprintf("steam://run/%v//%v/", appint, command),
		Command:  command,
	}

	// Load and parse the template file
	tmpl, err := template.ParseFiles(filepath.Join("www", "template.html"))
	if err != nil {
		http.Error(w, "Error loading template", http.StatusInternalServerError)
		return
	}

	// Render the template with data
	err = tmpl.Execute(w, data)
	if err != nil {
		http.Error(w, "Error rendering template", http.StatusInternalServerError)
		return
	}

	// Log the redirect
	cwlog.DoLog(false, "redirect: %v: %v", appName, command)
}
