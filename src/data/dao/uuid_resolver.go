package dao

import (
	"encoding/json"
	"os"
	"strings"
)

// xrayConfigClients espelha apenas o que precisamos do config.json do Xray.
// Funciona com vmess, vless e trojan (todos usam clients[].id + clients[].email).
type xrayConfigClients struct {
	Inbounds []struct {
		Settings struct {
			Clients []struct {
				ID    string `json:"id"`
				Email string `json:"email"`
			} `json:"clients"`
		} `json:"settings"`
	} `json:"inbounds"`
}

// Locais padrão do config.json do Xray no Linux.
var xrayConfigPaths = []string{
	"/usr/local/etc/xray/config.json",
	"/etc/xray/config.json",
	"/opt/xray/config.json",
}

// IsUUIDFormat retorna true se s tem o formato xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx (36 chars).
func IsUUIDFormat(s string) bool {
	return len(s) == 36 && s[8] == '-' && s[13] == '-' && s[18] == '-' && s[23] == '-'
}

// ResolveUUID lê o config.json do Xray e retorna o username (campo email)
// associado ao UUID fornecido. Retorna ("", false) se não encontrado.
func ResolveUUID(uuid string) (string, bool) {
	if !IsUUIDFormat(uuid) {
		return "", false
	}
	normalized := strings.ToLower(strings.TrimSpace(uuid))
	for _, path := range xrayConfigPaths {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		var cfg xrayConfigClients
		if err := json.Unmarshal(data, &cfg); err != nil {
			continue
		}
		for _, inbound := range cfg.Inbounds {
			for _, client := range inbound.Settings.Clients {
				if strings.ToLower(client.ID) == normalized && client.Email != "" {
					return client.Email, true
				}
			}
		}
	}
	return "", false
}
