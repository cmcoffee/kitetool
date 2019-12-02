package main

import (
	"fmt"
	"github.com/cmcoffee/go-ask"
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
		in := ask.ForInput(fmt.Sprintf(`
--- Proxy Configuration, Current Setting: %s

    [1] Set Proxy URI.
    [2] Disable Proxy, Use Direct Connections.

(selection or 'b' to go back): `, c.ProxyURI))

		switch in {
		case "1":
			proxy := ask.ForInput(`
# Proxy URI is typically in format of http://proxy_server.domain.com:3128
--> Proxy URI: `)
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
		in := ask.ForInput(fmt.Sprintf(`
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
			show_var(hide_var(cfg.Signature)), show_var(cfg.Admin), show_var(cfg.RedirectURI), cfg.SSLVerify, cfg.ProxyURI))
		switch in {
		case "1":
			cfg.Server = ask.ForInput(`
# Please provide the kiteworks appliance hostname. (ie.. kiteworks.domain.com)
--> kitworks Hostname: `)
			cfg.Tested = false
		case "2":
			cfg.ClientID = ask.ForInput(`
--> Client Application ID: `)
			cfg.Tested = false
		case "3":
			cfg.ClientSecret = ask.ForInput(`
--> Client Secret Key: `)
			cfg.Tested = false
		case "4":
			cfg.Signature = ask.ForInput(`
--> Signature Secret: `)
			cfg.Tested = false
		case "5":
			cfg.Admin = ask.ForInput(`
# Please provide the email of a system admin within the kiteworks appliance.
--> Set System Admin Login: `)
			cfg.Tested = false
		case "6":
			cfg.RedirectURI = ask.ForInput(`
# This simply needs to match the API configuration in kiteworks.
--> Redirect URI: `)
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
				if ask.ToConfirm("\nWould you like validate settings with a quick test?") {
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
