package testkit

import (
	"context"
	"sync"
)

// SentMessage records a delivered message.
type SentMessage struct {
	ChatID int64
	Text   string
}

// FakeSender is an in-memory CredentialSender for tests / verification.
type FakeSender struct {
	mu       sync.Mutex
	Err      error // when set, Send returns this error
	Messages []SentMessage
}

// NewFakeSender builds a fake sender.
func NewFakeSender() *FakeSender { return &FakeSender{} }

// Send records the message (or returns the configured error).
func (f *FakeSender) Send(_ context.Context, chatID int64, text string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.Err != nil {
		return f.Err
	}
	f.Messages = append(f.Messages, SentMessage{ChatID: chatID, Text: text})
	return nil
}

// Count returns how many messages were sent.
func (f *FakeSender) Count() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.Messages)
}

// SetErr configures the error returned by Send (nil to clear).
func (f *FakeSender) SetErr(err error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.Err = err
}
