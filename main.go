package main

import (
	"crypto/tls"
	"encoding/json"
	"flag"
	"fmt"
	"goRedirect/cwlog"
	"io"
	"log"
	"net/http"
	"os"
	"sync"
	"time"
)

const (
	timeOut        = 10 * time.Second
	updateInterval = time.Hour
	databaseName   = "apps.json"
)

var (
	bindIP        *string
	bindPortHTTPS *int
	bindPortHTTP  *int

	fileServer http.Handler
	lastUpdate time.Time

	database *dbData
	appList  map[uint64]*appData
	dbLock   sync.RWMutex
)

type dbData struct {
	AppList appListData `json:"applist"`
}
type appListData struct {
	Apps []*appData `json:"apps"`
}
type appData struct {
	Name  string `json:"name"`
	AppID uint64 `json:"appid"`
}

func redirect(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, "https://"+r.Host+r.RequestURI, http.StatusMovedPermanently)
}

func main() {
	bindIP = flag.String("ip", "", "IP to bind to")
	bindPortHTTPS = flag.Int("httpsPort", 443, "port to bind to for HTTPS")
	bindPortHTTP = flag.Int("httpPort", 80, "port to bind to")
	forceUpdate := flag.Bool("forceUpdate", false, "download the steamDB at launch.")

	flag.Parse()

	cwlog.StartLog()
	cwlog.LogDaemon()

	go func() {
		buf := fmt.Sprintf("%v:%v", *bindIP, *bindPortHTTP)
		if err := http.ListenAndServe(buf, http.HandlerFunc(redirect)); err != nil {
			log.Fatalf("ListenAndServe error: %v", err)
		}
	}()

	defer time.Sleep(time.Second)

	/* Download server */
	fileServer = http.FileServer(http.Dir("www"))

	appList = make(map[uint64]*appData)

	readDatabase(false)
	if *forceUpdate {
		updateDatabase(true)
	}
	lastUpdate = time.Now()

	/* Load certificates */
	cert, err := tls.LoadX509KeyPair("fullchain.pem", "privkey.pem")
	if err != nil {
		cwlog.DoLog(true, "Error loading TLS key pair: %v (fullchain.pem, privkey.pem)", err)
		return
	}
	cwlog.DoLog(true, "Loaded certs.")

	/* HTTPS server */
	http.HandleFunc("/", httpsHandler)

	/* Create TLS configuration */
	config := &tls.Config{
		Certificates:       []tls.Certificate{cert},
		InsecureSkipVerify: false,
	}

	/* Create HTTPS server */
	server := &http.Server{
		Addr:         fmt.Sprintf("%v:%v", *bindIP, *bindPortHTTPS),
		Handler:      http.DefaultServeMux,
		TLSConfig:    config,
		TLSNextProto: make(map[string]func(*http.Server, *tls.Conn, http.Handler), 0),

		ReadTimeout:  timeOut,
		WriteTimeout: timeOut,
		IdleTimeout:  timeOut,
	}

	go func() {
		for {
			time.Sleep(time.Minute)

			filePath := "fullchain.pem"
			initialStat, erra := os.Stat(filePath)

			if erra != nil {
				continue
			}

			for initialStat != nil {
				time.Sleep(time.Minute)

				stat, errb := os.Stat(filePath)
				if errb != nil {
					break
				}

				if stat.Size() != initialStat.Size() || stat.ModTime() != initialStat.ModTime() {
					cwlog.DoLog(true, "Cert updated, closing.")
					time.Sleep(time.Second * 5)
					os.Exit(0)
					break
				}
			}

		}
	}()

	// Start server
	cwlog.DoLog(true, "Starting server...")
	err = server.ListenAndServeTLS("", "")
	if err != nil {
		cwlog.DoLog(true, "ListenAndServeTLS: %v", err)
		panic(err)
	}

	cwlog.DoLog(true, "Goodbye.")
}

func readDatabase(verbose bool) {
	file, err := os.ReadFile(databaseName)
	if err != nil {
		cwlog.DoLog(true, "Error: Read database: %v", err)
		return
	}
	dbLock.Lock()
	err = json.Unmarshal(file, &database)
	dbLock.Unlock()

	if err != nil {
		cwlog.DoLog(true, "Error: Read database: %v", err)
		return
	}

	/* Add to map */
	dbLock.Lock()
	for _, app := range database.AppList.Apps {

		if appList[app.AppID] == nil {
			if verbose {
				cwlog.DoLog(true, "Added %v: %v", app.AppID, app.Name)
			}
		}
		appList[app.AppID] = app
	}
	dbLock.Unlock()

	lastUpdate = time.Now()
	cwlog.DoLog(true, "Read database.")
}

func updateDatabase(verbose bool) {

	cwlog.DoLog(true, "Updating database...")
	resp, err := http.Get("https://api.steampowered.com/ISteamApps/GetAppList/v0002/?key=STEAMKEY&format=json")
	if err != nil {
		cwlog.DoLog(true, "Error: Update database: %v", err)
		return
	}

	responseBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		cwlog.DoLog(true, "Error: Update database: %v", err)
		return
	}

	if len(responseBytes) < 4 {
		cwlog.DoLog(true, "Empty response from steam, aborting.")
		return
	}

	err = os.WriteFile(databaseName+".tmp", responseBytes, 0644)
	if err != nil {
		cwlog.DoLog(true, "Error: Update database: %v", err)
		return
	}

	err = os.Rename(databaseName+".tmp", databaseName)
	if err != nil {
		cwlog.DoLog(true, "Error: Update database: %v", err)
		return
	}

	readDatabase(verbose)
}
