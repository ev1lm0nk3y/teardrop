package main

// gdrive is the interface to Google Drive.

import (
	"fmt"
	"net/mail"
	"regexp"
	"strconv"
	"time"

	"google.golang.org/api/drive/v3"
	"google.golang.org/api/googleapi"
)

var defaultPermissionLevel = "reader"

// Allows for a specific yaml Unmarshaller to do validation of these fields
// differently than basic strings.
type recipient struct {
	Address string
}

func (r *recipient) UnmarshalYAML(b []byte) error {
	if _, err := mail.ParseAddress(string(b)); err != nil {
		return fmt.Errorf("email address invalid: %s", string(b))
	}
	r.Address = string(b)
	return nil
}

var (
	durationRegex = regexp.MustCompile(`(?<tWait>\d+)(?<tMod>[smhdw])`)
	day           = 24 * time.Hour
	week          = 7 * day
)

type duration struct {
	Wait time.Duration
}

func (d *duration) UnmarshalYAML(b []byte) error {
	m := durationRegex.FindStringSubmatch(string(b))
	ptw := parseDurationInt64(m[1])
	if ptw == 0 {
		return fmt.Errorf("unable to parse duration: %s", m[1])
	}
	dp := time.Duration(ptw)

	switch m[2] {
	case "s":
		d.Wait = time.Second * dp
	case "m":
		d.Wait = time.Minute * dp
	case "h":
		d.Wait = time.Hour * dp
	case "d":
		d.Wait = day * dp
	case "w":
		d.Wait = week * dp
	default:
		return fmt.Errorf("bad time modifier: %s", m[2])
	}

	return nil
}

func parseDurationInt64(w string) int64 {
	if len(w) == 0 {
		return 0
	}
	parsed, err := strconv.Atoi(w[:len(w)-1])
	if err != nil {
		return 0
	}
	return int64(parsed)
}

// Item is a Drive item to be sent to an email address after the delay has
// passed.
type Item struct {
	// Id references a Drive item Id. Get this from the webgui or gcloud cmdline
	Id string `yaml:"fileId"`
	// SendTo lists the email addresses to share this item with
	SendTo []recipient `yaml:"sendTo"`
	// SendDelay is a human-readable duration notation, i.e.
	//  1h = 1 hour
	//  1d = 1 day
	SendDelay duration `yaml:"sendDelay,omitempty"`
}

// Release will update the permission of the Item to reader for the recipient
// after the given delay.
func (i *Item) Release(service *drive.Service) *time.Timer {
	dps := drive.NewPermissionsService(service)
	return time.AfterFunc(i.SendDelay.Wait, func() { i.updatePermission(dps) })
}

func (i *Item) canIShare(dps *drive.FilesService) error {
	gf, err := dps.Get(i.Id).Do()
	if err != nil {
		return err
	}
	if gf.Permissions == nil {
		return fmt.Errorf("No access to share %s\n", gf.Name)
	}
	return nil
}

func (i *Item) updatePermission(dps *drive.PermissionsService) error {
	itemPerm := &drive.Permission{}
	for _, recvAddr := range i.SendTo {
		_, err := dps.Update(i.Id, defaultPermissionLevel, itemPerm).Do()
		if err != nil {
			if googleapi.IsNotModified(err) {
				fmt.Printf("%s is already allowed to view the document\n", recvAddr)
				break
			}
			return fmt.Errorf("Setting permission on id %s failed: %+v", i.Id, err)
		}
	}
	return nil
}
