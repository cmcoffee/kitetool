package main

import (
	"crypto/rand"
	"fmt"
	"github.com/cmcoffee/go-kvlite"
	"github.com/cmcoffee/go-nfo"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"time"
)

var loader = []string{
	"-\\-",
	"-|-",
	"-/-",
	"---",
}

type Error string

func (e Error) Error() string { return string(e) }

func init() {
	nfo.HideTS()
	errchk(nfo.File(nfo.STD, fmt.Sprintf("%s.log", APPNAME), 0, 0))
	global.start_time = time.Now()
	go func() {
		for {
			runtime_str := fmt.Sprintf("%s Running (%s)", APPNAME, time.Now().Sub(global.start_time).Round(time.Second).String())
			for _, str := range loader {
				if atomic.LoadInt32(&global.show_loader) == 1 {
					if global.snoop {
						goto Exit
					}
					nfo.Flash("%s %s %s", str, runtime_str, str)
				}
				time.Sleep(125 * time.Millisecond)
			}
		}
	Exit:
	}()
}

// Displays loader. "[>>>] Working, Please wait."
func ShowLoader() {
	atomic.CompareAndSwapInt32(&global.show_loader, 0, 1)
}

// Hides display loader.
func HideLoader() {
	atomic.CompareAndSwapInt32(&global.show_loader, 1, 0)
}

const (
	box = 1 << iota
	kiteworks
)

const (
	NONE  = ""
	SLASH = string(os.PathSeparator)
)

// Loggers
var (
	Log    = nfo.Log
	Fatal  = nfo.Fatal
	Notice = nfo.Notice
	Flash  = nfo.Flash
	Stdout = nfo.Stdout
	Stderr = nfo.Stderr
	Warn   = nfo.Warn
	Defer  = nfo.Defer
	Printf = nfo.Stdout
	Exit   = nfo.Exit
)

func Err(input ...interface{}) {
	global.errors.Add(1)
	nfo.Err(input...)
}

var path = filepath.Clean

type database struct {
	db *kvlite.Store
}

// DB Wrappers to perform fatal error checks on each call.
func (d database) Truncate(table string) {
	errchk(d.db.Truncate(table))
}

func (d database) CryptSet(table string, key, value interface{}) {
	errchk(d.db.CryptSet(table, key, value))
}

func (d database) Set(table string, key, value interface{}) {
	errchk(d.db.Set(table, key, value))
}

func (d database) Get(table string, key, output interface{}) bool {
	found, err := d.db.Get(table, key, output)
	errchk(err)
	return found
}

func (d database) ListKeys(table string) []string {
	keylist, err := d.db.ListKeys(table)
	errchk(err)
	return keylist
}

func (d database) ListNKeys(table string) []int {
	keylist, err := d.db.ListNKeys(table)
	errchk(err)
	return keylist
}

func (d database) Unset(table string, key interface{}) {
	errchk(d.db.Unset(table, key))
}

// Fatal Error Check
func errchk(err error) {
	if err != nil {
		Fatal(err)
	}
}

// Parse timestamps from box.com.
func read_box_time(input string) (time.Time, error) {
	if input == NONE {
		t := new(time.Time)
		return *t, nil
	}
	t, err := time.Parse(time.RFC3339, input)
	if err != nil {
		return t, err
	}
	return t, nil
}

// Parse Timestamps from kiteworks
func read_kw_time(input string) (time.Time, error) {
	input = strings.Replace(input, "+0000", "Z", 1)
	return time.Parse(time.RFC3339, input)
}

func write_kw_time(input time.Time) string {
	t := input.UTC().Format(time.RFC3339)
	return strings.Replace(t, "Z", "+0000", 1)
}

func gen_pass() string {
	return fmt.Sprintf("%s-abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789-%s-!@#$-%s-)^!*-%s-%%(~_-%s-+|{}", string(randBytes(4)), string(randBytes(4)), string(randBytes(4)), string(randBytes(4)), string(randBytes(4)))
}

// Generates a random byte slice of length specified.
func randBytes(sz int) []byte {
	if sz <= 0 {
		sz = 16
	}

	ch := "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789+/"
	chlen := len(ch)

	rand_string := make([]byte, sz)
	rand.Read(rand_string)

	for i, v := range rand_string {
		rand_string[i] = ch[v%byte(chlen)]
	}
	return rand_string
}

// Create a local folder
func MkDir(path string) (err error) {

	create := func(path string) (err error) {
		_, err = os.Stat(path)
		if err != nil && os.IsNotExist(err) {
			err = os.Mkdir(path, 0755)
			if err != nil {
				return err
			}
		} else if err != nil {
			return err
		}
		return nil
	}

	path = filepath.Clean(path)

	split_path := strings.Split(path, SLASH)
	for i, _ := range split_path {
		err = create(strings.Join(split_path[0:i+1], SLASH))
		if err != nil {
			return err
		}
	}

	return
}

func readDate(input string) (output time.Time, err error) {
	if input == NONE {
		return
	}
	output, err = time.Parse(time.RFC3339, fmt.Sprintf("%sT00:00:00Z", input))
	if err != nil {
		if strings.Contains(err.Error(), "parse") {
			err = fmt.Errorf("Invalid date specified, should be in format: YYYY-MM-DD")
		} else {
			err_split := strings.Split(err.Error(), ":")
			err = fmt.Errorf("Invalid date specified:%s", err_split[len(err_split)-1])
		}
	}
	return
}

//Writes the date out as a string.
func dateString(input time.Time) string {
	pad := func(i int) string {
		if i > 99 {
			return fmt.Sprintf("%d", i)
		}
		var out [2]byte
		if i < 10 {
			out = [2]byte{48, byte(48 + i)}
		} else {
			left := i / 10
			right := i % 10
			out = [2]byte{byte(48 + left), byte(48 + right)}
		}
		return string(out[0:])
	}

	return fmt.Sprintf("%s-%s-%s", pad(input.Year()), pad(int(input.Month())), pad(input.Day()))
}

// Provides human readable file sizes.
func showSize(bytes int64) string {

	names := []string{
		"Bytes",
		"KB",
		"MB",
		"GB",
	}

	suffix := 0
	size := float64(bytes)

	for size >= 1000 && suffix < len(names)-1 {
		size = size / 1000
		suffix++
	}

	return fmt.Sprintf("%.1f%s", size, names[suffix])
}

type stats_record int64

// Add number to stat record..
func (s *stats_record) Add(num int64) {
	atomic.StoreInt64((*int64)(s), atomic.AddInt64((*int64)(s), num))
}

// Set number for stat record..
func (s *stats_record) Set(num int64) {
	atomic.StoreInt64((*int64)(s), num)
}

// Get number from stat record.
func (s *stats_record) Get() int64 {
	return atomic.LoadInt64((*int64)(s))
}
