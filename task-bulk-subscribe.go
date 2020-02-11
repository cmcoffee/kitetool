package main

func init() {
	global.menu.Register("folder-notifications", "Set folder and file expiries for users.", BulkSubscribe)
}

func BulkSubscribe(flag *task) (err error) {
	fileAdded := flag.Bool("remove_file_notifications", true, "Subscribe to file notifications.")
	commentAdded := flag.Bool("remove_comment_notifications", true, "Subscribe to comment notifications.")
	folderList := flag.String("folder", "<folders to apply this to>", "Specify folders to run this on.")

	if err := flag.Parse(); err != nil {
		return err
	}

	my_func := func(user KiteUser) {
		S := KWSession(user.Email)

		for _, f := range BulkFolders(user, *folderList) {
			if f.Name == "My Folder" {
				continue
			}
			Log("[%s]: Updating notification settings for folder %s. (File Notifications: %v, Folder Notifications: %v)", string(S), f.Name, *fileAdded, *commentAdded)
			if err = S.SetNotifications(f.ID, true, *fileAdded, *commentAdded); err != nil {
				if !RestError(err, ERR_ACCESS_USER) {
					Err("[%s]: Cannot update notification settings for user as user is not active.", string(S))
					return
				}
			}
		}
	}

	return BulkAction(KiteUser{Deleted: false, Active: true, Suspended: false, Deactivated: false, Verified: true}, my_func)
}
