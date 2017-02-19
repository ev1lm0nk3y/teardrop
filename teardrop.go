package main

import (
	"bytes"
	"fmt"
	"io"
	"net/http"

	"google.golang.org/api/drive/v3"

	"github.com/subosito/twilio"

	"gopkg.in/yaml.v2"
)

var (
	hClient = &http.Client{}
)

// Config defines the teardrop configuration yaml. This should be stored either on
// disk where this is run, or on Google Drive. Loading of this file is done via
// command-line options.
type TDConfig struct {
	Frequency   duration `yaml:"Frequency"`
	ResponseTTL duration `yaml:"responseTTL"`
	Twilio      Twilio   `yaml:"twilio"`
	Items       []Item   `yaml:"files"`
}

func Load(input io.Reader) (*TDConfig, error) {
	var c TDConfig
	var b []byte
	cBuf := bytes.NewBuffer(b)
	if _, err := cBuf.ReadFrom(input); err != nil {
		return nil, err
	}
	if err := yaml.Unmarshal(cBuf.Bytes(), &c); err != nil {
		return nil, err
	}
	return &c, nil
}

func (tc *TDConfig) Validate() error {
	t := twilio.NewClient(tc.Twilio.SID, tc.Twilio.Token, nil)
	if t.AccountSid != "active" {
		return fmt.Errorf("Twilio account is disabled")
	}
	// check permissions on the items
	dc, err := drive.New(hClient)
	if err != nil {
		return err
	}
	fs := drive.NewFilesService(dc)
	for _, i := range tc.Items {
		if err := i.canIShare(fs); err != nil {
			return err
		}
	}
	return nil
}
