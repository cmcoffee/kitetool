package main

import (
	"fmt"
	"github.com/cmcoffee/go-nfo"
	"strings"
)

type Config struct {
	Admin        string `json:"admin_account"`
	Server       string `json:"server"`
	ClientID     string `json:"client_id"`
	ClientSecret string `json:"client_secret"`
	Signature    string `json:"signature"`
	Tested       bool   `json:"tested"`
	SSLVerify    bool   `json:"verify_ssl"`
	ProxyURI     string `json:"proxy_uri"`
	RedirectURI  string `json:"redirect_uri"`
}

const (
	no_proxy = "(Direct Connection/No Proxy)"
)

// Check if app is configured.
func (c Config) configured() bool {
	if c.Server == NONE || c.Admin == NONE || c.ClientID == NONE || c.ClientSecret == NONE || c.Signature == NONE {
		return false
	}
	return true
}

// Configuration for proxy settings.
func (c *Config) setup_proxy() {
	for {
		in := nfo.NeedAnswer(fmt.Sprintf(`
--- Proxy Configuration, Current Setting: %s

    [1] Set Proxy URI.
    [2] Disable Proxy, Use Direct Connections.

(selection or 'b' to go back): `, c.ProxyURI), nfo.Input)

		switch in {
		case "1":
			proxy := nfo.Input(`
# Proxy URI is typically in format of http://proxy_server.domain.com:3128
--> Proxy URI: `)
			if proxy == NONE {
				c.ProxyURI = no_proxy
				c.Tested = false
				continue
			}
			if !strings.Contains(proxy, "http") {
				Printf("\n*** Invalid URI '%s', URI should be in format of http://proxy_server.domain.com:3128.", proxy)
				continue
			} else {
				c.ProxyURI = proxy
				c.Tested = false
			}
			return
		case "2":
			c.ProxyURI = no_proxy
			return
		case "b":
			return
		default:
			Printf("\n*** Invalid option '%s', please try again.\n", in)
		}
	}
}

func test_api() (err error) {
	if global.config.ProxyURI == no_proxy {
		global.config.ProxyURI = NONE
		defer func() { global.config.ProxyURI = no_proxy }()
	}
	KWAdmin = KWSession(global.config.Admin)
	if _, err := KWAdmin.GetUsers(1, 1); err != nil {
		return err
	}
	return nil
}

// Perform setup of system.
func setup(setup_requested bool) {
	cfg := &global.config

	if !cfg.configured() {
		cfg.SSLVerify = true
		cfg.ProxyURI = no_proxy
		cfg.RedirectURI = fmt.Sprintf("https://%s/", APPNAME)
	} else if !setup_requested {
		if cfg.ProxyURI == no_proxy {
			cfg.ProxyURI = NONE
		}
		KWAdmin = KWSession(cfg.Admin)
		return
	}

	Defer(func() { Printf("\n") })

	hide_var := func(input string) string {
		var str []rune
		for _ = range input {
			str = append(str, '*')
		}
		return string(str)
	}

	show_var := func(input string) string {
		if input == NONE {
			return "*** UNCONFIGURED ***"
		} else {
			return input
		}
	}

	for {
		in := nfo.NeedAnswer(fmt.Sprintf(`
--- %s Configuration ---

  [1] kiteworks Host:   %s
  [2] Client App ID:    %s
  [3] Client Secret:    %s
  [4] Signature Secret: %s
  [5] SysAdmin Account: %s
  [6] Redirect URI:     %s
  [7] Verify SSL:       %v
  [8] Proxy Server:     %s

(selection or 'q' to save & exit): `, APPNAME, show_var(cfg.Server), show_var(cfg.ClientID), show_var(hide_var(cfg.ClientSecret)),
			show_var(hide_var(cfg.Signature)), show_var(cfg.Admin), show_var(cfg.RedirectURI), cfg.SSLVerify, cfg.ProxyURI), nfo.Input)
		switch in {
		case "1":
			cfg.Server = nfo.NeedAnswer(`
# Please provide the kiteworks appliance hostname. (ie.. kiteworks.domain.com)
--> kitworks Hostname: `, nfo.Input)
			cfg.Tested = false
		case "2":
			cfg.ClientID = nfo.NeedAnswer(`
--> Client Application ID: `, nfo.Input)
			cfg.Tested = false
		case "3":
			cfg.ClientSecret = nfo.NeedAnswer(`
--> Client Secret Key: `, nfo.Input)
			cfg.Tested = false
		case "4":
			cfg.Signature = nfo.NeedAnswer(`
--> Signature Secret: `, nfo.Input)
			cfg.Tested = false
		case "5":
			cfg.Admin = nfo.NeedAnswer(`
# Please provide the email of a system admin within the kiteworks appliance.
--> Set System Admin Login: `, nfo.Input)
			cfg.Tested = false
		case "6":
			cfg.RedirectURI = nfo.NeedAnswer(`
# This simply needs to match the API configuration in kiteworks.
--> Redirect URI: `, nfo.Input)
		case "7":
			if cfg.SSLVerify {
				cfg.SSLVerify = false
			} else {
				cfg.SSLVerify = true
			}
		case "8":
			cfg.setup_proxy()
		case "q":
			if !cfg.configured() {
				global.db.CryptSet(APPNAME, "config", &cfg)
				Exit(0)
			}
			if !cfg.Tested {
				if nfo.Confirm("\nWould you like validate settings with a quick test?") {
					if err := test_api(); err != nil {
						Err(err)
						continue
					} else {
						Printf("API Tested Successfully! Configuration has been updated.")
						cfg.Tested = true
						global.db.CryptSet(APPNAME, "config", &cfg)
						Exit(0)
					}
				} else {
					cfg.Tested = true
					global.db.CryptSet(APPNAME, "config", &cfg)
					Exit(0)
				}
			}
			global.db.CryptSet(APPNAME, "config", &cfg)
			Exit(0)
		case "?":
		default:
			Printf("\n*** Invalid option '%s', please try again.\n", in)
		}
	}
}
