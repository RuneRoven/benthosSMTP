package benthosSMTP

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"net/smtp"
	"strings"

	"github.com/redpanda-data/benthos/v4/public/service"
)

type loginAuth struct {
	username, password string
}

func LoginAuth(username, password string) smtp.Auth {
	return &loginAuth{username, password}
}

func (a *loginAuth) Start(server *smtp.ServerInfo) (string, []byte, error) {
	return "LOGIN", []byte{}, nil
}

func (a *loginAuth) Next(fromServer []byte, more bool) ([]byte, error) {
	if more {
		switch string(fromServer) {
		case "Username:":
			return []byte(a.username), nil
		case "Password:":
			return []byte(a.password), nil
		default:
			return nil, errors.New("Unknown fromServer")
		}
	}
	return nil, nil
}

// smtp structure
type smtpCommOutput struct {
	serverAddress      string       // smtp server address
	serverPort         int          // smtp server port
	senderAddress      string       // sender address
	recipients         []string     // List of recipients
	username           string       // username for the server
	password           string       // password for the server
	tls                string       // Start TLS option
	InsecureSkipVerify bool         //
	handler            *smtp.Client // handler for smtp client
}

var smtpConf = service.NewConfigSpec().
	Summary("Creates an output that sends data to smtp. Created by Daniel H").
	Description("This output plugin enables Benthos to send data via email using smtp. " +
		"Configure the plugin by specifying the stmp server address, port, sender address and recipients.").
	Field(service.NewStringField("serverAddress").Description("smtp server address.")).
	Field(service.NewIntField("serverPort").Description("server port. Default 25)").Default(25)).
	Field(service.NewStringField("senderAddress").Description("sender Address")).
	Field(service.NewStringListField("recipients").Description("List of recipients to send to")).
	Field(service.NewStringField("username").Description("username").Default("")).
	Field(service.NewStringField("password").Description("password").Default("")).
	Field(service.NewStringField("TLS").Description("STARTTLS or SMTPS").Default("")).
	Field(service.NewBoolField("InsecureSkipVerify").Description("InsecureSkipVerify").Default(false)).
	Field(service.NewOutputMaxInFlightField())

func newSmtpCommOutput(conf *service.ParsedConfig, mgr *service.Resources) (*smtpCommOutput, error) {
	serverAddress, err := conf.FieldString("serverAddress")
	if err != nil {
		return nil, err
	}

	serverPort, err := conf.FieldInt("serverPort")
	if err != nil {
		return nil, err
	}

	senderAddress, err := conf.FieldString("senderAddress")
	if err != nil {
		return nil, err
	}

	recipients, err := conf.FieldStringList("recipients")
	if err != nil {
		return nil, err
	}

	username, err := conf.FieldString("username")
	if err != nil {
		return nil, err
	}

	password, err := conf.FieldString("password")
	if err != nil {
		return nil, err
	}

	tls, err := conf.FieldString("TLS")
	if err != nil {
		return nil, err
	}

	InsecureSkipVerify, err := conf.FieldBool("InsecureSkipVerify")
	if err != nil {
		return nil, err
	}

	output := &smtpCommOutput{
		serverAddress:      serverAddress,
		serverPort:         serverPort,
		senderAddress:      senderAddress,
		recipients:         recipients,
		username:           username,
		password:           password,
		tls:                tls,
		InsecureSkipVerify: InsecureSkipVerify,
	}

	return output, nil
}

func init() {
	err := service.RegisterOutput(
		"smtp", smtpConf,
		func(conf *service.ParsedConfig, mgr *service.Resources) (out service.Output, maxInFlight int, err error) {
			if maxInFlight, err = conf.FieldMaxInFlight(); err != nil {
				return
			}
			out, err = newSmtpCommOutput(conf, mgr)
			return

		})
	if err != nil {
		panic(err)
	}
}

func (s *smtpCommOutput) Connect(ctx context.Context) error {
	if s.handler != nil {
		return nil
	}

	addr := fmt.Sprintf("%s:%d", s.serverAddress, s.serverPort)

	if s.tls != "" {
		tlsConfig := &tls.Config{
			ServerName:         s.serverAddress,
			InsecureSkipVerify: s.InsecureSkipVerify,
		}
		// Dial the SMTP server
		if s.tls == "STARTTLS" {
			conn, err := net.Dial("tcp", addr)
			if err != nil {
				return fmt.Errorf("net dial failed: %w", err)
			}
			client, err := smtp.NewClient(conn, s.serverAddress)
			if err != nil {
				return fmt.Errorf("failed to crete new client: %w", err)
			}
			err = client.StartTLS(tlsConfig)
			if err != nil {
				return fmt.Errorf("failed to start TLS: %w", err)
			}
			s.handler = client
		} else if s.tls == "SMTPS" {
			conn, err := tls.Dial("tcp", addr, tlsConfig)
			if err != nil {
				return fmt.Errorf("TLS dial failed: %w", err)
			}
			client, err := smtp.NewClient(conn, s.serverAddress)
			if err != nil {
				return fmt.Errorf("failed to crete new client: %w", err)
			}
			s.handler = client
		}
		auth := func() smtp.Auth {
			if s.tls == "STARTTLS" {
				return LoginAuth(s.username, s.password)
			} else {
				return smtp.PlainAuth("", s.username, s.password, s.serverAddress)
			}
		}()
		if err := s.handler.Auth(auth); err != nil {
			return fmt.Errorf("failed to authenticate with SMTP server: %w", err)
		}

	} else {
		client, err := smtp.Dial(addr)
		if err != nil {
			return fmt.Errorf("failed to dial SMTP server: %w", err)
		}
		// Set the client to the handler
		s.handler = client
	}

	return nil
}

func (s *smtpCommOutput) Write(ctx context.Context, msg *service.Message) error {
	if s.handler == nil {
		if err := s.Connect(ctx); err != nil {
			return err
		}
	}

	content, err := msg.AsStructured()
	if err != nil {
		return fmt.Errorf("failed to convert message to structured content: %v", err)
	}

	// Assert content as a map[string]interface{}
	msgMap, ok := content.(map[string]interface{})
	if !ok {
		return fmt.Errorf("unexpected type for content: %T", content)
	}

	// Access 'msg' field if it exists and is a string
	msgBody, ok := msgMap["msg"].(string)
	if !ok {
		return fmt.Errorf("msg field is not a string or does not exist")
	}
	msgSubject, ok := msgMap["subject"].(string)
	if !ok {
		//return fmt.Errorf("subject field is not a string or does not exist")
		msgSubject = "Default"
	}

	if err := s.handler.Mail(s.senderAddress); err != nil {
		return fmt.Errorf("failed to set sender address: %w", err)
	}

	for _, recipient := range s.recipients {
		if err := s.handler.Rcpt(recipient); err != nil {
			return fmt.Errorf("failed to set recipient address: %w", err)
		}
	}

	wc, err := s.handler.Data()
	if err != nil {
		return fmt.Errorf("failed to get data writer: %w", err)
	}
	defer wc.Close()

	headers := make(map[string]string)
	headers["From"] = s.senderAddress
	headers["To"] = strings.Join(s.recipients, ",")
	headers["Subject"] = msgSubject
	headers["Content-Type"] = "text/plain; charset=UTF-8"

	var emailBody strings.Builder
	for k, v := range headers {
		emailBody.WriteString(fmt.Sprintf("%s: %s\r\n", k, v))
	}
	// Access 'msg' field if it exists and is a string
	emailBody.WriteString("\r\n" + msgBody)
	if _, err := wc.Write([]byte(emailBody.String())); err != nil {
		return fmt.Errorf("failed to write email body: %w", err)
	}

	return nil
}

func (s *smtpCommOutput) Close(ctx context.Context) error {
	if s.handler != nil {
		if err := s.handler.Quit(); err != nil {
			return fmt.Errorf("failed to close SMTP connection: %w", err)
		}
		s.handler = nil
	}
	return nil
}
