package mail

import (
	"errors"
	"net/smtp"
	"strings"
)

type loginAuth struct {
	username, password string
}

// Start begins AUTH LOGIN and refuses it over plain text outside localhost.
func (a loginAuth) Start(server *smtp.ServerInfo) (string, []byte, error) {
	if server != nil && !server.TLS && !localhost(server.Name) {
		return "", nil, errors.New("unencrypted connection")
	}
	return "LOGIN", nil, nil
}

// Next answers the relay's username and password prompts.
func (a loginAuth) Next(fromServer []byte, more bool) ([]byte, error) {
	if !more {
		return nil, nil
	}
	switch strings.ToLower(strings.TrimSpace(strings.TrimSuffix(string(fromServer), ":"))) {
	case "username", "user name":
		return []byte(a.username), nil
	case "password":
		return []byte(a.password), nil
	default:
		return nil, errors.New("unexpected AUTH LOGIN prompt")
	}
}

func localhost(name string) bool {
	return name == "localhost" || name == "127.0.0.1" || name == "::1"
}
