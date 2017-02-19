package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"

	"github.com/subosito/twilio"
)

const (
	// Twilio MessageStatus' that are undesirable. Will need to check the
	// ErrorCode to ensure that this isn't a permanent issue.
	failedDelivery = "failed"
	undeliverable  = "undeliverable"

	// Twilio ErrorCodes that could signal a permenant delivery issue.
	accountSuspended   = 30002 // We won't be able to send anything out, abort
	unreachableHandset = 30003 // Could just be out of range, try again
	carrierViolation   = 30007 // Messages are being marked as spam and blocked
)

var (
	fatalSendError              = fmt.Errorf("Aborting! Unrecoverable send error")
	multipleSMSDeliveryFailures = fmt.Errorf("Too many delivery failures")
	retrySendError              = fmt.Errorf("Undeliverable message. Try again")

	mutex sync.RWMutex
)

type uri struct {
	Value string
}

func (u *uri) UnmarshalYAML(b []byte) error {
	if string(b[0]) != "/" {
		return fmt.Errorf("uri does not begin with a '/'")
	}
	return nil
}

type twilioErrMsg struct {
	twilio.Message
	ErrorCode    int
	ErrorMessage string
}

type Twilio struct {
	SID            string `yaml:"acctSID"`
	Token          string `yaml:"authToken"`
	CallbackURI    uri    `yaml:"callbackURI"`
	ResponseURI    uri    `yaml:"responseURI"`
	From           string `yaml:"from"`
	To             string `yaml:"to"`
	MaxUndelivered int    `yaml:"manUndelivered"`
}

type TwilioSend struct {
	MsgDeliveredChan chan bool
	MsgSid           string

	numUndelivered int
}

// Send an SMS message to your configured device. Return *TwilioSend to track
// message delivery.
func (t *Twilio) Send(msg string) (*TwilioSend, error) {
	tClient := twilio.NewClient(t.SID, t.Token, nil)
	msgParams := twilio.MessageParams{
		Body:           msg,
		StatusCallback: fmt.Sprintf("http://%s:%d%s", "127.0.0.1", 9823, t.CallbackURI),
	}
	ms, res, err := tClient.Messages.Send(t.From, t.To, msgParams)
	if err != nil {
		return nil, checkForErr(res.Body, res.StatusCode)
	}

	ts := &TwilioSend{
		MsgSid: ms.Sid,
	}
	ts.MsgDeliveredChan <- false
	return ts, nil
}

type SendMsg struct {
	Sid       string
	Failed    bool
	Delivered bool
}

var sChan = make(chan SendMsg, 10)

// SentMessageStatus is a REST endpoint for Twilio.CallbackURI for a specific
// message. Will set the channel to true if the message has been delivered.
func SentMessageStatus(req *http.Request, res *http.Response) {
	res.StatusCode = 200
	msg, err := parseTwilioMessage(req.Body, 0)
	if err != nil {
		res.StatusCode = 500
		return
	}
	s := SendMsg{Sid: msg.Sid}
	switch msg.Status {
	case "delivered":
		s.Delivered = true
	case "failed", "undelivered":
		s.Failed = true
	}
	sChan <- s
}

// Receive takes the request from the http server sent for incoming SMS messages.
type Receive struct {
	Number string
	Code   string
}

var rChan = make(chan Receive, 10)

// ReceiveSMS is a REST endpoint where Twilio will send messages that it
// receives on their configured numbers. It send the response, if it is
// properly formed, onto a channel to be processed elsewhere.
func ReceiveSMS(req *http.Request, res *http.Response) {
	res.StatusCode = 200
	msg, err := parseTwilioMessage(req.Body, 0)
	if err != nil {
		res.StatusCode = 500
		return
	}
	rChan <- Receive{
		Code:   msg.Body,
		Number: msg.From,
	}
}

func parseTwilioMessage(r io.ReadCloser, hs int) (*twilioErrMsg, error) {
	var b []byte
	var msg twilioErrMsg
	rBuf := bytes.NewBuffer(b)
	if _, err := rBuf.ReadFrom(r); err != nil {
		return nil, err
	}
	if err := json.Unmarshal(rBuf.Bytes(), &msg); err != nil {
		return nil, err
	}
	return &msg, nil
}

func checkForErr(b io.ReadCloser, statusCode int) error {
	if statusCode >= 300 {
		return fmt.Errorf("non-200 status code")
	}
	msg, err := parseTwilioMessage(b, statusCode)
	if err != nil {
		return err
	}
	if msg.Status == failedDelivery || msg.Status == undeliverable {
		if msg.ErrorCode == accountSuspended {
			return fatalSendError
		}
		return retrySendError
	}
	return nil
}
