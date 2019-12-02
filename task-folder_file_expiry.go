package main

import (
	"github.com/cmcoffee/go-nfo"
	"strings"
	"sync"
	"time"
	//"fmt"
)

func init() {
	global.menu.Register("folder-file-expiry", "Set folder and file expiries for users.", BulkFileExpire)
}

// Update folder & file expiration time.
func (b bulk_file_expiry) update_files_expiry(folder KiteFolder, folder_names []string) {

	S := b.KWSession

	folder_path := strings.Join(folder_names, "/")
	folder_days := b.folder_days

	const Day = time.Duration(time.Hour * 24)

	days_from_now := func(input time.Time) int {
		return int(input.UTC().Sub(time.Now().UTC()).Hours() / 24)
	}

	var (
		cur_folder_expiry, new_folder_expiry time.Time
		err                                  error
	)

	switch e := folder.Expire.(type) {
	case int:
	case string:
		cur_folder_expiry, err = read_kw_time(e)
		if err != nil {
			nfo.Err(err)
			return
		}
	}

	// Filter out folders that match our --min-expiry and --max-expiry
	if len(folder_names) == 1 {
		cur := cur_folder_expiry.Unix()
		if b.min_date_set {
			if !cur_folder_expiry.IsZero() && !b.min_date.IsZero() && cur < b.min_date.Unix() {
				return
			}
			if !cur_folder_expiry.IsZero() && b.min_date.IsZero() {
				return
			}
		}
		if b.max_date_set {
			if !cur_folder_expiry.IsZero() && !b.max_date.IsZero() && b.max_date.Unix() < cur {
				return
			}
			if cur_folder_expiry.IsZero() && !b.max_date.IsZero() {
				return
			}
		}
	}

	if folder_days > 0 {
		folder_days++
		new_folder_expiry = time.Now().Add(Day * time.Duration(folder_days-1))
	}

	original_file_days := folder.FileLifetime

	// Retain existing folder expiration if it is higher than what is set.
	if !cur_folder_expiry.IsZero() && !new_folder_expiry.IsZero() {
		if cur_folder_expiry.Unix() > new_folder_expiry.Unix() && b.only_extend {
			new_folder_expiry = cur_folder_expiry
			folder_days = days_from_now(cur_folder_expiry)
		}
	} else if cur_folder_expiry.IsZero() && b.only_extend {
		new_folder_expiry = cur_folder_expiry
	}

	// If reduce_expiry = yes, lower file expiry to configured file expiry.
	if !b.only_extend && folder.FileLifetime > b.file_days && b.file_days != 0 {
		folder.FileLifetime = b.file_days
	}

	if folder.FileLifetime > 0 && b.file_days > folder.FileLifetime || folder.FileLifetime == 0 && !b.only_extend {
		folder.FileLifetime = b.file_days
	}

	// If file lifetime is higher than folder expiry, set file expiry to folder expiry.
	if folder_days > 0 && folder.FileLifetime > folder_days {
		folder.FileLifetime = folder_days - 1
	}

	if b.file_days == 0 {
		folder.FileLifetime = 0
	}

	if b.only_reduce {
		if !cur_folder_expiry.IsZero() && (new_folder_expiry.Unix() > cur_folder_expiry.Unix() || new_folder_expiry.IsZero()) {
			new_folder_expiry = cur_folder_expiry
		}

		if original_file_days > 0 && (folder.FileLifetime > original_file_days || folder.FileLifetime == 0) {
			folder.FileLifetime = original_file_days
		}
	}

	if !b.only_extend_files {
		if !new_folder_expiry.IsZero() {
			if folder.FileLifetime == 0 {
				nfo.Log("%s [%d]: Folder Expiry: %v - File Expiry: Expires with folder.", folder_path, folder.ID, dateString(new_folder_expiry))
			} else {
				nfo.Log("%s [%d]: Folder Expiry: %v - File Expiry: %d days.", folder_path, folder.ID, dateString(new_folder_expiry), folder.FileLifetime)
			}
		} else {
			if folder.FileLifetime == 0 {
				nfo.Log("%s [%d]: Folder Expiry: Never expires - File Expiry: Never expires.", folder_path, folder.ID)
			} else {
				nfo.Log("%s [%d]: Folder Expiry: Never expires - File Expiry: %d days.", folder_path, folder.ID, folder.FileLifetime)
			}
		}

		// Set folder expiration
		if !new_folder_expiry.IsZero() {
			err = S.SetFolderAndFileExpiry(folder.ID, new_folder_expiry, folder.FileLifetime)
		} else {
			err = S.SetFolderAndFileExpiry(folder.ID, 0, folder.FileLifetime)
		}

		if err != nil {
			nfo.Err("%s [%d]: %s", folder_path, folder.ID, err.Error())
			return
		}
	} else {
		date_string := dateString(cur_folder_expiry)
		if cur_folder_expiry.IsZero() {
			date_string = "Never expires."
		}
		nfo.Log("%s [%d]: Reapplying folder's file expirations to files. [folder: %s file_exp:%d days]", folder_path, folder.ID, date_string, original_file_days)
		if err := S.ReapplyFileLifetime(folder.ID); err != nil {
			nfo.Err(err)
		}
	}

	nested_folders, err := S.ListFolders(folder.ID)
	if err != nil {
		nfo.Err(err)
		return
	}
	for _, f := range nested_folders {
		folder_names = append(folder_names, f.Name)
		if b.checkout_folder(f, false) {
			b.update_files_expiry(f, folder_names)
		}
	}

}

// Prevents folders with multiple users from modifying the same folders.
func (b *bulk_file_expiry) checkout_folder(folder KiteFolder, check_permissions bool) bool {

	User := b.KWSession

	b.folder_mutex.Lock()
	defer b.folder_mutex.Unlock()

	if b.work_folders == nil {
		b.work_folders = make(map[int]struct{})
	}

	if _, ok := b.work_folders[folder.ID]; ok {
		return false
	}

	if check_permissions {
		f, err := User.FolderInfo(folder.ID)
		if err != nil {
			nfo.Err(err)
			return false
		}

		if f.CurrentUserRole.ID >= 4 {
			b.work_folders[folder.ID] = struct{}{}
			return true
		} else {
			if f.CurrentUserRole.ID == 0 {
				nfo.Warn("Unable to check user role for user %s of folder %s.", string(User), string(folder.Name))
				return true
			}
			nfo.Log("%s is not a owner nor manager of %s... Skipping for now.", string(User), string(folder.Name))
			return false
		}
	}

	b.work_folders[folder.ID] = struct{}{}
	return true
}

type bulk_file_expiry struct {
	folder_mutex      sync.Mutex
	work_folders      map[int]struct{}
	folder_days       int
	file_days         int
	only_reduce       bool
	only_extend       bool
	max_date_set      bool
	min_date_set      bool
	max_date          time.Time
	min_date          time.Time
	only_extend_files bool

	KWSession
}

func BulkFileExpire(flag *task) (err error) {

	folders := flag.String("folders", "<top-level folder>", "Specific folders to modify expiry for.")
	folder_days := flag.Int("folder-expiry-days", 0, "Folder expiration in days, 0 for no expiration.")
	file_days := flag.Int("file-expiry-days", 0, "File expiration in days, 0 for expire with folder expiration.")
	only_extend := flag.Bool("only-extend", false, "Only extend expiry, do not reduce expiry on folders and files")
	only_reduce := flag.Bool("only-reduce", false, "Only reduce expiration, do not extend expiry on folders and files.")
	only_extend_files := flag.Bool("apply-file-expiry", false, "Only extend file expirations to current folder/file expirations.")
	min_date_str := flag.String("min-expiry", "<YYYY-MM-DD>", "Only process folders with expiry above min date. (0 for never expires)")
	max_date_str := flag.String("max-expiry", "<YYYY-MM-DD>", "Only process folders with expiry below max date. (0 for never expires")
	if err := flag.Parse(); err != nil {
		return err
	}

	if (!flag.IsSet("folder-expiry-days") || !flag.IsSet("file-expiry-days")) && !flag.IsSet("apply-file-expiry") {
		return Error("You must specify either: --folder-expiry-days and --file-expiry-days OR --apply-file-expiry")
	}

	if flag.IsSet("only-extend") && flag.IsSet("only-reduce") {
		return Error("--only-extend and --only-reduce are mutually exclusive options.")
	}

	select_folders := strings.Split(*folders, ",")
	if select_folders[0] == NONE {
		select_folders = select_folders[0:0]
	}

	if len(select_folders) > 0 && !flag.IsSet("user") {
		return Error("--user is mandatory modifier when specifying specific --folders.")
	}

	var max_date, min_date time.Time

	if *max_date_str != "0" {
		max_date, err = readDate(*max_date_str)
		if err != nil {
			return err
		}
		max_date = max_date.Add(time.Duration(time.Hour*23) + time.Duration(time.Minute*59) + time.Duration(time.Second*59))
	}

	if *min_date_str != "0" {
		min_date, err = readDate(*min_date_str)
		if err != nil {
			return err
		}
	}

	if flag.IsSet("max-expiry") && flag.IsSet("min-expiry") {
		if !max_date.IsZero() && max_date.Unix() < min_date.Unix() || min_date.IsZero() && !max_date.IsZero() {
			return Error("--min-expiry cannot be greater than --max-expiry.")
		}
	}

	if *folder_days != 0 && *folder_days < *file_days {
		return Error("--folder-expiry-days cannot be lower than --files-expiry-days.")
	}

	flag.LogStart()

	// Hand-off function to BulkAction.
	my_func := func(user KiteUser) {

		if user.BaseDirID == 0 {
			return
		}

		b := &bulk_file_expiry{
			folder_days:       *folder_days,
			file_days:         *file_days,
			only_reduce:       *only_reduce,
			only_extend:       *only_extend,
			only_extend_files: *only_extend_files,
			max_date:          max_date,
			min_date:          min_date,
			KWSession:         KWSession(user.Email),
		}

		if flag.IsSet("max-expiry") {
			b.max_date_set = true
		}

		if flag.IsSet("min-expiry") {
			b.min_date_set = true
		}

		if len(select_folders) == 0 {
			folders, err := b.GetFolders()
			if err != nil {
				nfo.Err("%v: Unable to process user: %s", b.KWSession, err.Error())
				return
			}

			for _, f := range folders {
				if b.checkout_folder(f, true) {
					if f.Name == "My Folder" {
						continue
					}
					b.update_files_expiry(f, []string{f.Name})
				}
			}
		} else {
			for _, folder_name := range select_folders {
				folder_name = strings.TrimPrefix(folder_name, "/")
				folder_name = strings.TrimPrefix(folder_name, "\\")
				if strings.Contains(folder_name, "/") || strings.Contains(folder_name, "\\") {
					nfo.Err("[%s]: %s contains a nested folder, folders should be top-level only, skipping folder.", b.KWSession, folder_name)
					continue
				}
				folder_id, err := b.FindFolder(folder_name)
				if err != nil {
					if RestError(err, ERR_ACCESS_USER) || err == ErrNotFound {
						return
					}
					nfo.Err("[%s]: %s, skipping folder.", folder_name, err.Error())
					continue
				}
				f, err := b.FolderInfo(folder_id)
				if err != nil {
					nfo.Err("[%s]: %s, skipping folder.", folder_name, err.Error())
					continue
				}
				if b.checkout_folder(f, true) {
					if f.Name == "My Folder" {
						continue
					}
					b.update_files_expiry(f, []string{folder_name})
				}
			}
		}
	}
	return BulkAction(KiteUser{Deleted: false, Active: true, Suspended: false, Deactivated: false, Verified: true}, my_func)
}
