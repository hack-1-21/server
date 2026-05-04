package models

import "time"

type StartDeviceLinkResponse struct {
	DeviceID         string `json:"device_id"`
	PairingCode      string `json:"pairing_code"`
	ExpiresInSeconds int    `json:"expires_in_seconds"`
}

type CompleteDeviceLinkRequest struct {
	Code string `json:"code"`
}

type PollDeviceLinkRequest struct {
	DeviceID string `json:"device_id"`
}

type PollDeviceLinkResponse struct {
	Status      string `json:"status"`
	DeviceToken string `json:"device_token,omitempty"`
}

type LinkedDevice struct {
	DeviceID   string     `json:"device_id"`
	LinkedAt   time.Time  `json:"linked_at"`
	LastUsedAt *time.Time `json:"last_used_at,omitempty"`
}
