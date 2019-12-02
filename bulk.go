package main

import (
	"sync"
)

// Bulk process for handling process call back against multiple users.
func BulkAction(user_filter KiteUser, process func(user KiteUser)) error {
	ShowLoader()
	defer HideLoader()
	s := KWAdmin

	wg := new(sync.WaitGroup)

	var i int

	for {
		var (
			u   []KiteUser
			err error
		)
		if len(global.user_list) == 0 {
			u, err = s.GetUsers(100, i)
			if err != nil {
				return err
			}

			count := len(u)
			if count == 0 {
				break
			}
			i = i + count
		} else {
			for _, email := range global.user_list {
				user_info, err := s.FindUser(email)
				if err != nil {
					Err("Unable to process user '%s': %s", email, err.Error())
					continue
				}
				u = append(u, *user_info)
			}
		}
		for _, user := range u {
			if user.Deleted != user_filter.Deleted || user.Active != user_filter.Active || user.Suspended != user_filter.Suspended || user.Deactivated != user_filter.Deactivated {
				continue
			}

			if !global.snoop {
				wg.Add(1)
				go func(user KiteUser) {
					defer wg.Done()
					process(user)
					UnsetUserCache(user)
				}(user)
			} else {
				process(user)
				UnsetUserCache(user)
			}
		}
		wg.Wait()
		if len(global.user_list) > 0 {
			break
		}
	}
	return nil
}
