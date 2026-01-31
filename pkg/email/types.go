// Package email provides types and interfaces for email operations.
package email

import (
	"time"
)

// Message represents an email message.
type Message struct {
	UID         uint32       `json:"uid,omitempty"`
	SeqNum      uint32       `json:"seq_num,omitempty"`
	MessageID   string       `json:"message_id,omitempty"`
	Subject     string       `json:"subject"`
	From        string       `json:"from"`
	To          []string     `json:"to"`
	CC          []string     `json:"cc,omitempty"`
	BCC         []string     `json:"bcc,omitempty"`
	Date        time.Time    `json:"date"`
	Body        string       `json:"body,omitempty"`
	BodyPreview string       `json:"body_preview,omitempty"`
	Attachments []Attachment `json:"attachments,omitempty"`
	Flags       []string     `json:"flags,omitempty"`
}

// Attachment represents an email attachment.
type Attachment struct {
	Filename    string `json:"filename"`
	ContentType string `json:"content_type"`
	Size        int    `json:"size"`
}

// SendRequest represents a request to send an email.
type SendRequest struct {
	From        string            `json:"from"`
	To          []string          `json:"to"`
	CC          []string          `json:"cc,omitempty"`
	BCC         []string          `json:"bcc,omitempty"`
	Subject     string            `json:"subject"`
	Body        string            `json:"body"`
	HTMLBody    string            `json:"html_body,omitempty"`
	Attachments []string          `json:"attachments,omitempty"`
	Headers     map[string]string `json:"headers,omitempty"`
}

// SendResponse represents the response from sending an email.
type SendResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message,omitempty"`
	Error   string `json:"error,omitempty"`
}

// InboxResponse represents the response for inbox listing.
type InboxResponse struct {
	Success  bool      `json:"success"`
	Messages []Message `json:"messages,omitempty"`
	Total    int       `json:"total"`
	Error    string    `json:"error,omitempty"`
}

// ReadResponse represents the response for reading an email.
type ReadResponse struct {
	Success bool    `json:"success"`
	Message Message `json:"message,omitempty"`
	Error   string  `json:"error,omitempty"`
}
