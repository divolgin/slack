package slack

import (
	"log"
	"time"

	"github.com/pkg/errors"
	"github.com/slack-go/slack"
)

type SlackProgress struct {
	client *slack.Client

	UserToken    string
	SlackChannel string

	StatusPrefix string
	Animation    []string
	StatusString string

	resetChan      chan interface{}
	StopChan       chan interface{}
	ErrorChan      chan error
	CurrentMessage *SlackMessage
}

type SlackMessage struct {
	Text    string
	Channel string
	Ts      string
}

type SlackCreateResponse struct {
	Channel string `json:"channel"`
	Ts      string `json:"ts"`
}

type SlackUpdateResponse struct {
	Channel string `json:"channel"`
	Ts      string `json:"ts"`
	Text    string `json:"text"`
}

type SlackHistoryResponse struct {
	Latest   string          `json:"latest"`
	Messages []slack.Message `json:"messages"`
}

func (p *SlackProgress) Start() {
	p.client = slack.New(p.UserToken)

	p.StopChan = make(chan interface{}, 1)
	p.ErrorChan = make(chan error, 1)
	if p.Animation == nil {
		p.Animation = []string{"|", "/", "--", "\\", "|", "/", "--", "\\"}
	}
	go p.runProgress()
}

func (p *SlackProgress) runProgress() {
	p.resetChan = make(chan interface{}, 1)

	text := p.StatusPrefix
	spinnerIdx := 0
	if len(p.Animation) > 0 {
		text += p.Animation[spinnerIdx]
	}
	response, err := p.createMessage(text)
	if err != nil {
		p.ErrorChan <- err
		return
	}

	p.CurrentMessage = &SlackMessage{
		Text:    text,
		Channel: response.Channel,
		Ts:      response.Ts,
	}

	defer func() {
		p.deleteMessage(p.CurrentMessage.Ts, p.CurrentMessage.Channel)
		p.CurrentMessage = nil
	}()
	go p.monitorHistory()

	for {
		select {
		case <-time.After(500 * time.Millisecond):
			spinnerFrame := ""
			if len(p.Animation) > 0 {
				spinnerIdx = (spinnerIdx + 1) % len(p.Animation)
				spinnerFrame = p.Animation[spinnerIdx]
			}
			_, err := p.updateMessage("*" + p.StatusPrefix + "* *" + spinnerFrame + "* ```" + p.StatusString + "```")
			if err != nil {
				p.ErrorChan <- err
				return
			}
		case <-p.StopChan:
			return
		case <-p.resetChan:
			go p.runProgress()
			return
		}
	}
}

func (p *SlackProgress) monitorHistory() {
	for {
		select {
		case <-p.StopChan:
			return

		case <-time.After(5 * time.Second):
			if p.CurrentMessage == nil {
				continue
			}

			response, err := p.channelHistory()
			if err != nil {
				log.Println("Error getting history, messages will not be moved:", err)
				return
			}

			if len(response.Messages) > 2 {
				close(p.resetChan)
				return
			}
		}
	}
}

func (p *SlackProgress) createMessage(text string) (*SlackCreateResponse, error) {
	channel, timestamp, err := p.client.PostMessage(p.SlackChannel, slack.MsgOptionText(text, false))
	if err != nil {
		return nil, errors.Wrap(err, "post message")
	}

	return &SlackCreateResponse{
		Channel: channel,
		Ts:      timestamp,
	}, nil
}

func (p *SlackProgress) updateMessage(text string) (*SlackUpdateResponse, error) {
	channel, ts, text, err := p.client.UpdateMessage(p.CurrentMessage.Channel, p.CurrentMessage.Ts, slack.MsgOptionText(text, false))
	if err != nil {
		return nil, errors.Wrap(err, "update message")
	}

	return &SlackUpdateResponse{
		Channel: channel,
		Ts:      ts,
		Text:    text,
	}, nil
}

func (p *SlackProgress) deleteMessage(ts, channel string) error {
	_, _, err := p.client.DeleteMessage(channel, ts)
	if err != nil {
		return errors.Wrap(err, "delete message")
	}

	return nil
}

func (p *SlackProgress) channelHistory() (*SlackHistoryResponse, error) {
	history, err := p.client.GetConversationHistory(&slack.GetConversationHistoryParameters{
		ChannelID: p.SlackChannel,
		Oldest:    p.CurrentMessage.Ts,
		Limit:     3,
	})
	if err != nil {
		return nil, errors.Wrap(err, "get conversation history")
	}

	return &SlackHistoryResponse{
		Latest:   history.Latest,
		Messages: history.Messages,
	}, nil
}
