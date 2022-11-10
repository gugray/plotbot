package main

import "time"

type Account struct {
	Account     string `json:"acct"`
	DisplayName string `json:"display_name"`
}

type MediaAttachment struct {
	Type string
	URL  string
}

type Update struct {
	ID               string
	CreatedAt        time.Time `json:"created_at"`
	URL              string
	Content          string
	Account          Account
	MediaAttechments []MediaAttachment `json:"media_attachments"`
}
