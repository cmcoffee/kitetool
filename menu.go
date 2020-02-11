package main

import (
	"fmt"
	"github.com/cmcoffee/go-eflag"
	"os"
	"sort"
	"strings"
	"sync"
	"text/tabwriter"
)

// Menu for tasks.
type menu struct {
	mutex   sync.RWMutex
	text    *tabwriter.Writer
	entries map[string]*task
}

// Registers a task with the task menu.
func (m *menu) Register(name, desc string, exec func(*task) error, required_params ...string) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	if m.entries == nil {
		m.entries = make(map[string]*task)
	}
	m.entries[name] = &task{
		name:     name,
		desc:     desc,
		exec:     exec,
		required: required_params[0:],
		EFlagSet: eflag.NewFlagSet(fmt.Sprintf("%s", name), eflag.ReturnErrorOnly),
	}
	my_entry := m.entries[name]
	my_entry.EFlagSet.Header = fmt.Sprintf("desc: \"%s\"\n", desc)
	my_entry.BoolVar(&global.snoop, "snoop", false, "")
	my_entry.users = my_entry.EFlagSet.String("user", "<user@domain.com>", "Single out users for specified task, use comma seperated value for multi-user.")
}

// Write out menu item.
func (m *menu) menu_write(cmd string, desc string) {
	if m.text == nil {
		m.text = tabwriter.NewWriter(os.Stderr, 34, 8, 1, ' ', 0)
	}
	m.text.Write([]byte(fmt.Sprintf("  %s\t%s\n", cmd, desc)))
}

// Completed menu.
func (m *menu) fin() {
	if m.text == nil {
		m.text = tabwriter.NewWriter(os.Stderr, 34, 8, 1, ' ', 0)
	}
	m.text.Write([]byte(fmt.Sprintf("\n")))
	m.text.Flush()
}

// Read menu items.
func (m *menu) Show() {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	os.Stderr.Write([]byte(fmt.Sprintf("\nAvailable '%s' commands:\n", APPNAME)))
	var items []string
	for k, _ := range m.entries {
		items = append(items, k)
	}
	sort.Strings(items)
	for _, k := range items {
		m.menu_write(k, m.entries[k].desc)
	}
	m.fin()
	os.Stderr.Write([]byte(fmt.Sprintf("For extended help on any task, type %s <command> --help.\n", APPNAME)))
}

// Select a specific task.
func (m *menu) Select(args []string) (err error) {
	if len(args) == 0 {
		m.Show()
	} else {
		m.mutex.RLock()
		if x, ok := m.entries[args[0]]; ok {
			x.args = args[1:]
			m.mutex.RUnlock()

			if err := x.exec(x); err != nil {
				if err != eflag.ErrHelp {
					Stderr("[ERROR] %s\n\n", err.Error())
				}
				x.EFlagSet.Usage()
				os.Exit(1)
			}
		} else {
			m.mutex.RUnlock()
			return fmt.Errorf("[ERROR] No such task: '%s' found.\n\n", args[0])
		}
	}
	return
}

// Menu item.
type task struct {
	name     string
	desc     string
	exec     func(*task) error
	required []string
	args     []string
	users    *string
	*eflag.EFlagSet
}

// Message to show when task is starting.
func (m *task) LogStart() {
	Log("--> %s '%s' started..", APPNAME, m.name)
	Log("\n")
}

// Prase flags assocaited with task.
func (m *task) Parse() (err error) {
	if err = m.EFlagSet.Parse(m.args[0:]); err != nil {
		return err
	}

	if m.users != nil {
		for _, u := range strings.Split(*m.users, ",") {
			if u != NONE {
				global.user_list = append(global.user_list, u)
			}
		}
	}

	for i, v := range m.required {
		if m.IsSet(v) {
			m.required = m.required[i+1:]
		}
	}

	for i, v := range m.required {
		m.required[i] = fmt.Sprintf("--%s", v)
	}

	if len(m.required) > 0 {
		Stderr("[ERROR] Missing manadatory arguments: %v\n", strings.Join(m.required, ", "))
		m.Usage()
		Exit(1)
	}
	return nil
}
