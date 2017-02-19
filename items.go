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
	if _, err := mail.ParseAddress(addr); err != nil {
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

	switch m[2] {
	case "s":
		d.Wait = ptw * time.Second
	case "m":
		d.Wait = ptw * time.Minute
	case "h":
		d.Wait = ptw * time.Hour
	case "d":
		d.Wait = ptw * day
	case "w":
		d.Wait = ptw * week
	default:
		return fmt.Errorf("bad time modifier: %s", m[2])
	}

	return nil
}

func parseDurationInt64(w string) int64 {
	if len(w) == 0 {
		return 0
	}
	parsed, err := strconv.Atoi(value[:len(w)-1])
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
	dps := service.NewPermissionsService()
	return time.AfterFunc(i.SendDelay.Wait, i.updatePermission(dps))
}

func (i *Item) updatePermission(dps *drive.PermissionService) error {
	itemPerm := &drive.Permission{}
	for _, recvAddr := range i.SendTo {
		p, err := dps.Update(i.Id, defaultPermissionLevel, itemPerm).Do()
		if err != nil {
			if googleapis.IsNotModified(err) {
				fmt.Printf("%s is already allowed to view the document\n", recvAddr)
				break
			}
			return fmt.Errorf("Setting permission on id %s failed: %+v", i.Id, err)
		}
	}
	return nil
}
