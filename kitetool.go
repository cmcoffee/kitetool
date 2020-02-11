package main

import (
	"fmt"
	"github.com/cmcoffee/go-eflag"
	"github.com/cmcoffee/go-kvlite"
	"github.com/cmcoffee/go-nfo"
	"os"
	"strings"
	"sync"
	"time"
)

var global struct {
	kw_server   string
	snoop       bool
	timeout     time.Duration
	db          database
	cache       database
	show_loader int32
	start_time  time.Time
	config      Config
	user_list   []string
	menu        menu
	errors      stats_record
	mutex       sync.Mutex
}

var api_call_bank = make(chan interface{}, MAX_CONNECTIONS)

const (
	MAX_CONNECTIONS = 3
	MAX_RETRY       = 3
	timeout         = time.Duration(time.Second * 100)
)

const (
	APPNAME       = "kitetool"
	RELEASE_YEAR  = 20
	RELEASE_MONTH = 2
	RELEASE_MINOR = 0
)

var VERSION_STRING = fmt.Sprintf("%d.%d.%d", RELEASE_YEAR, RELEASE_MONTH, RELEASE_MINOR)

func init() {
	for i := 0; i < MAX_CONNECTIONS; i++ {
		api_call_bank <- struct{}{}
	}
	var err error
	global.cache.db, err = kvlite.MemStore()
	errchk(err)
}

func main() {
	var err error
	defer nfo.Exit(0)
	Defer(HideLoader)

	global.db.db, err = kvlite.Open(fmt.Sprintf("%s.db", APPNAME))
	errchk(err)

	header := fmt.Sprintf("### %s Admin Assistant/v%s ###\n\n", APPNAME, VERSION_STRING)

	flag := eflag.NewFlagSet(os.Args[0], eflag.ReturnErrorOnly)
	//flag.Header = fmt.Sprintf("-- %s kiteworks Admin Assistant (%v)\n", APPNAME, VERSION_STRING)
	setup_requested := flag.Bool("setup", false, "Configure API settings for kiteworks appliance.")

	if err := flag.Parse(os.Args[1:]); err != nil {
		if err != eflag.ErrHelp {
			Stderr("[ERROR] %s\n\n", err.Error())
		} else {
			Stderr(header)
		}
		flag.Usage()
		global.menu.Show()
		os.Exit(0)
	}

	Defer(global.db.db.Close)

	global.db.Get(APPNAME, "config", &global.config)

	request_help := false

	for _, x := range flag.Args() {
		if strings.Contains(strings.ToLower(x), "--help") || strings.Contains(strings.ToLower(x), "-h") {
			request_help = true
		}
	}

	if len(os.Args) < 2 {
		Stderr(header)
		flag.Usage()
		global.menu.Show()
		os.Exit(0)
	} else {
		if !request_help {
			// Load API Configuration
			setup(*setup_requested)
		}

		if err = global.menu.Select(flag.Args()); err != nil {
			Stderr(err.Error())
			flag.Usage()
			global.menu.Show()
		} else {
			Log("\n")
			Log("Process completed in %s with %d errors.", time.Now().Sub(global.start_time).Round(time.Second).String(), global.errors)
		}
	}
}
