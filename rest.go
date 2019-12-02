package main

import (
	"strings"
	"time"
)

const ErrNotFound = Error("Requested object could not be found.")

// Kiteworks Links Data
type KiteLinks struct {
	Relationship string `json:"rel"`
	Entity       string `json:"entity"`
	ID           int    `json:"id"`
	URL          string `json:"href"`
}

// Kiteworks Folder Data
type KiteFolder struct {
	ID              int              `json:"id"`
	Created         string           `json:"created"`
	Deleted         bool             `json:"deleted"`
	Expire          interface{}      `json:"expire"`
	Modified        string           `json:"modified"`
	Name            string           `json:"name"`
	Description     string           `json:"description"`
	ParentID        int              `json:"parentId"`
	UserID          int              `json:"userId"`
	Permalink       string           `json:"permalink"`
	Locked          int              `json:"locked"`
	Status          string           `json:"status"`
	FileLifetime    int              `json:"fileLifetime"`
	Type            string           `json:"type"`
	Links           []KiteLinks      `json:"links"`
	MailID          int              `json:"mail_id"`
	CurrentUserRole FolderPermission `json:"currentUserRole"`
}

// Kiteworks File Data
type KiteFile struct {
	ID              int              `json:"id"`
	Created         string           `json:"created"`
	Modified        string           `json:"modified"`
	Deleted         bool             `json:"deleted"`
	PermDeleted     bool             `json:"permDeleted"`
	ClientCreated   string           `json:"clientCreated"`
	ClientModified  string           `json:"clientModified"`
	Expire          interface{}      `json:"expire"`
	Name            string           `json:"name"`
	Description     string           `json:"description"`
	ParentID        int              `json:"parentId"`
	UserID          int              `json:"userId"`
	Permalink       string           `json:"permalink"`
	Locked          int              `json:"locked"`
	Fingerprint     string           `json:"fingerprint"`
	Status          string           `json:"status"`
	Size            int64            `json:"size"`
	Mime            string           `json:"mime"`
	Quarantined     bool             `json:"quarantined"`
	DLPLocked       bool             `json:"dlpLocked"`
	FileLifetime    int              `json:"fileLifetime"`
	Type            string           `json:"type"`
	Links           []KiteLinks      `json:"links"`
	CurrentUserRole FolderPermission `json:"currentUserRole"`
}

// Permission information
type FolderPermission struct {
	ID         int    `json:"id"`
	Name       string `json:"name"`
	Rank       int    `json:"rank"`
	Modifiable bool   `json:"modifiable"`
	Disabled   bool   `json:"disabled"`
}

// List Folders.
func (s KWSession) ListFolders(folder_id int) (output []KiteFolder, err error) {

	var KiteArray struct {
		Folders []KiteFolder `json:"data"`
	}

	req := APIRequest{
		APIVer: 7,
		Method: "GET",
		Path:   SetPath("/rest/folders/%d/folders", folder_id),
		Params: SetParams(Query{"deleted": false}),
		Output: &KiteArray,
	}
	return KiteArray.Folders, s.Call(req)
}

// List Files.
func (s KWSession) ListFiles(folder_id int) ([]KiteFile, error) {

	var KiteArray struct {
		Files []KiteFile `json:"data"`
	}

	req := APIRequest{
		APIVer: 5,
		Method: "GET",
		Path:   SetPath("/rest/folders/%d/files", folder_id),
		Params: SetParams(Query{"deleted": false}),
		Output: &KiteArray,
	}
	return KiteArray.Files, s.Call(req)
}

// Pulls up all top level folders.
func (s KWSession) GetFolders() ([]KiteFolder, error) {

	var KiteArray struct {
		Folders []KiteFolder `json:"data"`
	}

	req := APIRequest{
		Method: "GET",
		Path:   "/rest/folders/top",
		Params: SetParams(Query{"deleted": false}),
		Output: &KiteArray,
	}
	return KiteArray.Folders, s.Call(req)
}

// Set expiries on folder.
func (s KWSession) SetFolderAndFileExpiry(folder_id int, expiry interface{}, fileLifetime int) (err error) {

	var Params []interface{}

	switch e := expiry.(type) {
	case int:
		Params = SetParams(PostJSON{"expire": 0, "fileLifetime": fileLifetime, "applyFileLifetimeToFiles": true})
	case time.Time:
		Params = SetParams(PostJSON{"expire": dateString(e), "fileLifetime": fileLifetime, "applyFileLifetimeToFiles": true})
	}

	req := APIRequest{
		APIVer: 13,
		Method: "PUT",
		Path:   SetPath("/rest/folders/%d", folder_id),
		Params: Params,
	}

	return s.Call(req)
}

func (s KWSession) ReapplyFileLifetime(folder_id int) error {
	req := APIRequest{
		Method: "PUT",
		Path:   SetPath("/rest/folders/%d", folder_id),
		Params: SetParams(PostJSON{"applyFileLifetimeToFiles": true}),
	}
	return s.Call(req)
}

// Returns Folder information.
func (s KWSession) FolderInfo(folder_id int) (output KiteFolder, err error) {
	req := APIRequest{
		Method: "GET",
		Path:   SetPath("/rest/folders/%d", folder_id),
		Params: SetParams(Query{"mode": "full", "deleted": false, "with": "(currentUserRole, fileLifetime)"}),
		Output: &output,
	}
	return output, s.Call(req)
}

// Returns the folder id of folder, can be specified as TopFolder/Nested or TopFolder\Nested.
func (s KWSession) FindFolder(remote_folder string) (id int, err error) {

	id = -1

	base_folder_id, err := s.MyBaseDirID()
	if err != nil {
		return -1, err
	}

	folder_names := strings.Split(remote_folder, "\\")
	if len(folder_names) == 1 {
		folder_names = strings.Split(remote_folder, "/")
	}

	shift_name := func() bool {
		if len(folder_names) > 1 {
			folder_names = folder_names[1:]
			return true
		}
		return false
	}

	var folders []KiteFolder

	if base_folder_id < 1 {
		folders, err = s.GetFolders()
		if err != nil {
			return
		}
	} else {
		folders, err = s.ListFolders(base_folder_id)
		if err != nil {
			return
		}
	}

	for _, e := range folders {
		if strings.ToLower(e.Name) == strings.ToLower(folder_names[0]) {
			id = e.ID
			break
		}
	}

	if id < 0 {
		return -1, ErrNotFound
	}

	for shift_name() {
		found := false
		nested, err := s.ListFolders(id)
		if err != nil {
			return -1, err
		}

		for _, elem := range nested {
			if strings.ToLower(elem.Name) == strings.ToLower(folder_names[0]) {
				id = elem.ID
				found = true
				break
			}
		}

		if !found {
			return -1, ErrNotFound
		}
	}

	return
}
