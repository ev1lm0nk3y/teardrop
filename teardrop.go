package main

import (
	"bytes"
	"context"
	"io"
	"net/http"

	"google.golang.org/api/drive/v3"
	"gopkg.in/yaml.v2"
)

// TD defines the teardrop configuration yaml. This should be stored either on
// disk where this is run, or on Google Drive. Loading of this file is done via
// command-line options.
type TearDrop struct {
	CheckDuration duration `yaml:"checkDuration"`
	Twilio        Twilio   `yaml:"twilio"`
	Items         []Item   `yaml:"files"`

	gDriveClient *drive.Service
}

func LoadTearDropConfig(tdConfig io.Reader) (*TD, error) {
	td := &TD{}
	var b []byte
	tdBuf := bytes.NewBuffer(b)
	if _, err := tdBuf.ReadFrom(tdConfig); err != nil {
		return nil, err
	}
	if err := yaml.Unmarshal(tdBuf, td); err != nil {
		return nil, err
	}
	return td, nil
}

func (td *TD) Run() {
	hc := &http.Client{}
	var err error
	if td.driveClient, err = drive.New(hc); err != nil {
		return err
	}

	for {
		time.Wait(td.CheckDuration.Wait)
		td.Twilio.SendRequest(td.SMS)
	}
}

func (td *TD) GetDriveService(hc *http.Client) error {
	panic("NI")
}
