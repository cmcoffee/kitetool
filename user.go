package main

import (
	"fmt"
	"strings"
)

// Kiteworks User Data
type KiteUser struct {
	ID          int    `json:"id"`
	Active      bool   `json:"active"`
	Deactivated bool   `json:"deactivated"`
	Suspended   bool   `json:"suspended"`
	BaseDirID   int    `json:"basedirId"`
	Deleted     bool   `json:"deleted"`
	Email       string `json:"email"`
	MyDirID     int    `json:"mydirId"`
	Name        string `json:"name"`
	SyncDirID   int    `json:"syncdirId"`
	UserTypeID  int    `json:"userTypeId"`
	Verified    bool   `json:"verified"`
	Internal    bool   `json:"internal"`
}

// Store KW User in cache.
func SetUserCache(input *KiteUser) {
	email := strings.ToLower(input.Email)
	global.cache.Set("kw_users", email, input)
	global.cache.Set("kw_user_id_map", input.ID, email)
}

func UnsetUserCache(input KiteUser) {
	global.cache.Unset("kw_users", strings.ToLower(input.Email))
	global.cache.Unset("kw_user_id_map", input.ID)
}

// Lookup KW User in cache
func (s *KWSession) KWUser(input interface{}) (output *KiteUser, err error) {
	var found bool

	switch in := input.(type) {
	case int:
		var email string
		if found = global.cache.Get("kw_user_id_map", in, &email); found {
			found = global.cache.Get("kw_users", email, &output)
		}
		if found {
			return
		} else {
			output, err = s.userInfo(in)
		}
	case string:
		if found = global.cache.Get("kw_users", strings.ToLower(in), &output); found {
			return
		} else {
			output, err = s.findUser(in)
		}
	case KWSession:
		if found = global.cache.Get("kw_users", strings.ToLower(string(in)), &output); found {
			return
		} else {
			output, err = s.findUser(string(in))
		}
	}
	return
}

// Get user information.
func (s KWSession) userInfo(user_id int) (output *KiteUser, err error) {
	err = s.Call(APIRequest{
		Method: "GET",
		Path:   SetPath("/rest/users/%d", user_id),
		Output: &output,
	})
	if err == nil {
		SetUserCache(output)
	}
	return
}

// Find a user_id
func (s KWSession) findUser(user_email string) (kw_user *KiteUser, err error) {
	user_email = strings.ToLower(user_email)

	var info struct {
		Users []KiteUser `json:"data"`
	}

	req := APIRequest{
		Method: "GET",
		Path:   "/rest/users",
		Params: SetParams(Query{"email": user_email}),
		Output: &info,
	}

	err = s.Call(req)
	if err != nil {
		return nil, err
	}

	if len(info.Users) == 0 {
		return nil, fmt.Errorf("No such user.")
	}

	SetUserCache(&info.Users[0])
	return &info.Users[0], nil
}

// Get My User information.
func (s KWSession) MyUser() (output *KiteUser, err error) {
	if output, _ = s.KWUser(s); output != nil {
		return output, nil
	}

	req := APIRequest{
		Method: "GET",
		Path:   "/rest/users/me",
		Output: &output,
	}

	err = s.Call(req)
	if err == nil {
		SetUserCache(output)
	}

	return output, s.Call(req)
}

// Get MyDirID
func (s KWSession) MyMyDirID() (folder_id int, err error) {
	out, err := s.MyUser()
	if err != nil {
		return -1, err
	}
	return out.MyDirID, nil
}

// Returns Folder ID of the Account's My Folder.
func (s KWSession) MyBaseDirID() (file_id int, err error) {
	out, err := s.MyUser()
	if err != nil {
		return -1, err
	}
	return out.BaseDirID, nil
}

// Returns User Information
func (s KWSession) GetUsers(limit, offset int) (output []KiteUser, err error) {

	var OutputArray struct {
		Users []KiteUser `json:"data"`
	}

	req := APIRequest{
		Method: "GET",
		Path:   SetPath("/rest/admin/users"),
		Params: SetParams(Query{"limit": limit, "offset": offset, "allowsCollaboration": true}),
		Output: &OutputArray,
	}

	return OutputArray.Users, s.Call(req)

}
