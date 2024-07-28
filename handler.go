package main

import (
	"fmt"
	"goRedirect/cwlog"
	"goRedirect/sclean"
	"html"
	"net/http"
	"strconv"
	"strings"
	"time"

	strip "github.com/grokify/html-strip-tags-go"
)

const (
	title          = "Redirect"
	redirectPrefix = "/gosteam/"
	maxUrlLen      = 128
)

func httpsHandler(w http.ResponseWriter, r *http.Request) {

	if r.Method != http.MethodGet {
		return
	}
	if !strings.HasPrefix(r.URL.Path, redirectPrefix) {
		fileServer.ServeHTTP(w, r)
		return
	}

	/* Stop if URL is big */
	if len(r.URL.Path) > maxUrlLen {
		return
	}
	input := html.UnescapeString(r.URL.Path)
	input = strip.StripTags(input)
	args := strings.SplitN(input, ".", 2)
	if len(args) != 2 {
		w.Write([]byte("Usage: https://go-game.net/gosteam/123456.command\nResult: steam://run/123456//command/"))
		cwlog.DoLog(true, "Invalid number of arguments: %v", input)
		return
	}

	appinput, command := strings.TrimPrefix(args[0], redirectPrefix), args[1]
	appint, err := strconv.ParseUint(appinput, 10, 64)
	if err != nil {
		return
	}

	appName := ""
	dbLock.RLock()
	appData := appList[appint]
	dbLock.RUnlock()

	if appData != nil {
		appName = sclean.StripControlAndSpecial(appData.Name)
	} else {
		/* Update and try to find it, if we haven't updated recently */
		if time.Since(lastUpdate) > updateInterval {
			dbLock.Lock()
			updateDatabase(true)
			appData = appList[appint]
			dbLock.Unlock()
		}

		/* Handle found and not found */
		if appData == nil {
			appName = "UNKNOWN steam appid: " + strconv.FormatUint(appint, 10)
		} else {
			appName = sclean.StripControlAndSpecial(appData.Name)
		}
	}

	steamURL := fmt.Sprintf("steam://run/%v//%v/", appint, command)

	launch := fmt.Sprintf("<a href=\"%v\">Connect with %v</a><br>will run command:<br>'%v'", steamURL, appName, command)
	lookup := fmt.Sprintf("<br><br><a href=\"https://steamdb.info/app/%v/\">Lookup appid</a><br>(steamdb.info)", appint)
	footer := "<br><br>Brought to you by:<br><a href=\"https://go-game.net/\">go-game.net</a>"
	body := launch + lookup + footer

	main := "<!DOCTYPE html><html><header><title>%v</title><style>body {color: #ffffff;background-color: #1f1f1f;font-size: 20px;line-height: 1.42857143;font-family: Lato;text-align:center;}</style></header><body><br>%v</body></html>"
	htmlText := fmt.Sprintf(main, title, body)

	w.Write([]byte(htmlText))
	cwlog.DoLog(false, "redirect: %v: %v", appName, command)
}
