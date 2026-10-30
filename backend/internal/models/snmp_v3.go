package models

import (
	"errors"
	"fmt"
)

// SNMPv3 security levels (RFC 3414).
const (
	SNMPv3NoAuthNoPriv = "noAuthNoPriv"
	SNMPv3AuthNoPriv   = "authNoPriv"
	SNMPv3AuthPriv     = "authPriv"
)

// SNMPv3AuthProtocols and SNMPv3PrivProtocols are the names accepted in
// SNMPv3Config; they match gosnmp's protocol names.
var (
	SNMPv3AuthProtocols = []string{"MD5", "SHA", "SHA224", "SHA256", "SHA384", "SHA512"}
	SNMPv3PrivProtocols = []string{"DES", "AES", "AES192", "AES256", "AES192C", "AES256C"}
)

// SNMPv3Config is a device's USM credential set. The passphrases are
// secrets: API responses mask them and readers never receive the block.
type SNMPv3Config struct {
	SecurityName   string `json:"securityName"`
	SecurityLevel  string `json:"securityLevel"`
	AuthProtocol   string `json:"authProtocol,omitempty"`
	AuthPassphrase string `json:"authPassphrase,omitempty"`
	PrivProtocol   string `json:"privProtocol,omitempty"`
	PrivPassphrase string `json:"privPassphrase,omitempty"`
	ContextName    string `json:"contextName,omitempty"`
}

func contains(list []string, v string) bool {
	for _, s := range list {
		if s == v {
			return true
		}
	}
	return false
}

// Validate checks that the fields required by the security level are set.
// USM requires passphrases of at least 8 characters.
func (c *SNMPv3Config) Validate() error {
	if c.SecurityName == "" {
		return errors.New("snmpV3.securityName is required")
	}
	if len(c.SecurityName) > 32 {
		return errors.New("snmpV3.securityName must be at most 32 characters")
	}
	switch c.SecurityLevel {
	case SNMPv3NoAuthNoPriv:
		return nil
	case SNMPv3AuthNoPriv, SNMPv3AuthPriv:
	default:
		return fmt.Errorf("snmpV3.securityLevel must be one of %s, %s, %s", SNMPv3NoAuthNoPriv, SNMPv3AuthNoPriv, SNMPv3AuthPriv)
	}
	if !contains(SNMPv3AuthProtocols, c.AuthProtocol) {
		return fmt.Errorf("snmpV3.authProtocol must be one of %v", SNMPv3AuthProtocols)
	}
	if len(c.AuthPassphrase) < 8 {
		return errors.New("snmpV3.authPassphrase must be at least 8 characters")
	}
	if c.SecurityLevel == SNMPv3AuthNoPriv {
		return nil
	}
	if !contains(SNMPv3PrivProtocols, c.PrivProtocol) {
		return fmt.Errorf("snmpV3.privProtocol must be one of %v", SNMPv3PrivProtocols)
	}
	if len(c.PrivPassphrase) < 8 {
		return errors.New("snmpV3.privPassphrase must be at least 8 characters")
	}
	return nil
}
