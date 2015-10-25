package slack

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type SlackProgress struct {
	UserToken      string
	SlackChannel   string
	MessagePrefix  string
	StopChan       chan interface{}
	ErrorChan      chan error
	CurrentMessage *SlackMessage
}

type SlackMessage struct {
	Test     string `json:"text"`
	Username string `json:"username"`
	Type     string `json:"type"`
	Subtype  string `json:"subtype"`
	Ts       string `json:"ts"`
}

type SlackCreateResponse struct {
	Ok      bool         `json:"ok"`
	Channel string       `json:"channel"`
	Ts      string       `json:"ts"`
	Message SlackMessage `json:"message"`
	Error   string       `json:"error"`
}

type SlackUpdateResponse struct {
	Ok      bool   `json:"ok"`
	Channel string `json:"channel"`
	Ts      string `json:"ts"`
	Text    string `json:"text"`
	Error   string `json:"error"`
}

type SlackHistoryResponse struct {
	Ok       bool           `json:"ok"`
	Latest   string         `json:"latest"`
	Messages []SlackMessage `json:"messages"`
	HasMore  bool           `json:"has_more"`
}

var httpClient = http.Client{}

func (p *SlackProgress) Start() {
	p.StopChan = make(chan interface{}, 1)
	p.ErrorChan = make(chan error, 1)
	go p.runProgress()
}

func (p *SlackProgress) runProgress() {
	spinners := []string{"|", "/", "--", "\\", "|", "/", "--", "\\"}
	response, err := p.createMessage(p.MessagePrefix + spinners[0])
	if err != nil {
		p.ErrorChan <- err
		return
	}

	if !response.Ok {
		p.ErrorChan <- fmt.Errorf("Could not send message to slack: %s", response.Error)
		return
	}

	p.CurrentMessage = &response.Message

	defer func() {
		p.deleteMessage(p.CurrentMessage.Ts, response.Channel)
		p.CurrentMessage = nil
	}()
	go p.monitorHistory(response.Channel)

	spinnerIdx := 1
	for {
		select {
		case <-time.After(500 * time.Millisecond):
			_, err := p.updateMessage(response.Message.Ts, response.Channel, p.MessagePrefix+spinners[spinnerIdx])
			if err != nil {
				p.ErrorChan <- err
				return
			}
			spinnerIdx = (spinnerIdx + 1) % len(spinners)
		case <-p.StopChan:
			return
		}
	}
}

func (p *SlackProgress) monitorHistory(channel string) {
	for {
		time.Sleep(5 * time.Second)
		if p.CurrentMessage == nil {
			continue
		}

		response, err := p.channelHistory(p.CurrentMessage.Ts, channel)
		if err != nil {
			// TODO: logging
			return
		}

		if !response.Ok {
			// TODO: logging
			return
		}

		if len(response.Messages) > 2 {
			p.StopChan <- nil
			time.Sleep(1 * time.Second) // there's a tiny race condition here
			go p.runProgress()
			return
		}
	}
}

func (p *SlackProgress) createMessage(text string) (*SlackCreateResponse, error) {
	url := fmt.Sprintf("https://slack.com/api/chat.postMessage?token=%s&channel=%s&text=%s&pretty=1&as_user=1",
		url.QueryEscape(p.UserToken),
		url.QueryEscape(p.SlackChannel),
		url.QueryEscape(text))
	body, err := callSlack(url)
	if err != nil {
		return nil, err
	}

	var response SlackCreateResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, err
	}

	return &response, nil
}

func (p *SlackProgress) updateMessage(ts, channel, text string) (*SlackUpdateResponse, error) {
	url := fmt.Sprintf("https://slack.com/api/chat.update?token=%s&channel=%s&ts=%s&text=%s",
		url.QueryEscape(p.UserToken),
		url.QueryEscape(channel),
		url.QueryEscape(ts),
		url.QueryEscape(text))
	body, err := callSlack(url)
	if err != nil {
		return nil, err
	}

	var response SlackUpdateResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, err
	}

	return &response, nil
}

func (p *SlackProgress) deleteMessage(ts, channel string) error {
	url := fmt.Sprintf("https://slack.com/api/chat.delete?token=%s&channel=%s&ts=%s",
		url.QueryEscape(p.UserToken),
		url.QueryEscape(channel),
		url.QueryEscape(ts))
	_, err := callSlack(url)
	if err != nil {
		return err
	}

	return nil
}

func (p *SlackProgress) channelHistory(ts, channel string) (*SlackHistoryResponse, error) {
	// Why can't Slack use the same function for channels and people???
	apiFunc := "im.history"
	if strings.HasPrefix(p.SlackChannel, "#") {
		apiFunc = "channels.history"
	}

	url := fmt.Sprintf("https://slack.com/api/%s?token=%s&channel=%s&oldest=%s&count=3",
		apiFunc,
		url.QueryEscape(p.UserToken),
		url.QueryEscape(channel),
		url.QueryEscape(ts))
	body, err := callSlack(url)
	if err != nil {
		return nil, err
	}

	var response SlackHistoryResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, err
	}

	return &response, nil
}

func callSlack(url string) ([]byte, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	return body, nil
}
