package main

func init() {
	global.menu.Register("mail-cleanup", "Performs cleaning of user's mail folder.", mail_cleaner)
}

// Cleans up older drafts and their files.
func mail_cleaner(flag *task) (err error) {

	expiry := flag.String("expire-drafts", "<YYYY-MM-DD>", "Expire drafts and their files older than specified date in UTC.")
	if err := flag.Parse(); err != nil {
		return err
	}

	expire, err := readDate(*expiry)
	if err != nil {
		return err
	}

	type KiteMail struct {
		Data []struct {
			ID int `json:"id"`
		} `json:"data"`
	}

	flag.LogStart()

	var drafts_deleted, files_deleted, files_total_size stats_record
	mail_files_cleaner_func := purge_deleted_mail_files(&files_deleted, &files_total_size)

	my_func := func(user KiteUser) {
		S := KWSession(user.Email)
		var mail_ids []int
		var mail KiteMail
		var offset int
		for {
			err = S.Call(APIRequest{
				Method: "GET",
				Path:   "/rest/mail",
				Params: SetParams(Query{"deleted": false, "offset": offset, "limit": 100, "bucket": "draft", "date:lte": write_kw_time(expire)}),
				Output: &mail,
			})
			if err != nil {
				return
			}
			for _, v := range mail.Data {
				mail_ids = append(mail_ids, v.ID)
			}
			if len(mail.Data) < 100 {
				break
			}
		}
		defer mail_files_cleaner_func(user)

		if len(mail_ids) == 0 {
			Log("[%v]: No expired drafts were found.", S)
			return
		}
		if len(mail_ids) > 0 {
			if err := S.Call(APIRequest{
				Method: "DELETE",
				Path:   "/rest/mail",
				Params: SetParams(Query{"emailId:in": mail_ids, "partialSuccess": true}),
			}); err != nil {
				Err("[%s]: Error deleting drafts: %s", string(S), err.Error())
			}
			Log("[%v]: Overdue drafts removed: %d", S, len(mail_ids))
			drafts_deleted.Add(int64(len(mail_ids)))
		}
	}
	if flag.IsSet("expire-drafts") {
		err = BulkAction(KiteUser{Deleted: false, Active: true, Suspended: false, Deactivated: false, Verified: true}, my_func)
		if err != nil {
			return err
		}
	}
	if !flag.IsSet("expire-drafts") {
		err = BulkAction(KiteUser{Deleted: false, Active: true, Suspended: false, Deactivated: false, Verified: true}, mail_files_cleaner_func)
		if err != nil {
			return err
		}
	}
	Log("\n")
	Log("    -- Runtime Totals --")
	if flag.IsSet("expire-drafts") {
		Log("      Drafts Removed: %d", drafts_deleted.Get())
	}
	Log(" Attachments Deleted: %d", files_deleted.Get())
	Log("   Storage Recovered: %s", showSize(files_total_size.Get()))
	return
}

// Deletes purged files from mail folder
func purge_deleted_mail_files(files_deleted, files_total_size *stats_record) func(user KiteUser) {

	return func(user KiteUser) {
		S := KWSession(user.Email)

		folder_id, err := S.MyMyDirID()
		if err != nil {
			Err("[%v]: Could not retrieve user's mail folder: %s", S, err.Error())
			return
		}

		if folder_id == 0 {
			return
		}

		var KiteArray struct {
			Files []KiteFile `json:"data"`
		}

		var offset int
		var deleted_files []int
		var total_size int64
		var file_count int

		for {
			err = S.Call(APIRequest{
				APIVer: 5,
				Method: "GET",
				Path:   SetPath("/rest/folders/%d/files", folder_id),
				Params: SetParams(Query{"deleted": true, "offset": offset, "limit": 100}),
				Output: &KiteArray,
			})
			if err != nil {
				Err("[%v]: Error while retriving files from mail dir: %s", S, err.Error())
				return
			}
			for _, v := range KiteArray.Files {
				if !v.PermDeleted {
					deleted_files = append(deleted_files, v.ID)
					total_size = total_size + v.Size
					file_count++
				}
			}
			if len(KiteArray.Files) < 100 {
				break
			}
		}
		if len(deleted_files) == 0 {
			Log("[%v]: No deleted attachments found in mail folder.", S)
			return
		}
		Log("[%v]: Purging %d deleted attachments from mail folder. (%s)", S, file_count, showSize(total_size))
		err = S.Call(APIRequest{
			Method: "DELETE",
			Path:   "/rest/files/actions/permanent",
			Params: SetParams(Query{"id:in": deleted_files, "partialSuccess": true}),
		})
		if err != nil {
			Err("[%v]: Error purging deleted attachments: %s", S, err.Error())
		}
		files_deleted.Add(int64(file_count))
		files_total_size.Add(total_size)
	}
}
