package app

import "time"

const Version = "2.2.1"

type Settings struct {
	TallyURL    string `json:"tallyUrl"`
	TimeoutSec  int    `json:"timeoutSec"`
	CooldownSec int    `json:"cooldownSec"`
	Port        int    `json:"port"`
	PDFTemplate string `json:"pdfTemplate,omitempty"`
	CloudflaredPath string `json:"cloudflaredPath,omitempty"`
	TunnelName string `json:"tunnelName,omitempty"`
	TunnelHostname string `json:"tunnelHostname,omitempty"`
	AutoSyncMinutes int `json:"autoSyncMinutes,omitempty"`
	CacheRetentionDays int `json:"cacheRetentionDays,omitempty"`
	DefaultBankName string `json:"defaultBankName,omitempty"`
	DefaultBankAccount string `json:"defaultBankAccount,omitempty"`
	DefaultBankIFSC string `json:"defaultBankIFSC,omitempty"`
	DefaultUPI string `json:"defaultUPI,omitempty"`
	UpdateChannel string `json:"updateChannel,omitempty"`
	UpdateRepo string `json:"updateRepo,omitempty"`
	AutoStart bool `json:"autoStart,omitempty"`
	LastCompany string `json:"lastCompany,omitempty"`
}

type Device struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	PublicKey   string    `json:"publicKey"`
	Fingerprint string    `json:"fingerprint"`
	CreatedAt   time.Time `json:"createdAt"`
	LastSeen    time.Time `json:"lastSeen"`
	Revoked     bool      `json:"revoked"`
}
