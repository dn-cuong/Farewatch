package alerts

import (
	"crypto/tls"
	"fmt"
	"net"
	"net/smtp"
	"strings"
	"time"
)

type Mailer struct {
	host string
	port string
	from string
	user string
	pass string
}

func NewMailer(host, port, from, user, pass string) *Mailer {
	return &Mailer{host: host, port: port, from: from, user: user, pass: pass}
}

type DropEmail struct {
	To          string
	Origin      string
	Destination string
	DepartDate  string
	Airline     string
	OldPrice    float64
	NewPrice    float64
	DeepLink    string
}

func (m *Mailer) SendDropAlert(msg DropEmail) (time.Duration, error) {
	start := time.Now()
	dropPct := 0.0
	if msg.OldPrice > 0 {
		dropPct = (msg.OldPrice - msg.NewPrice) / msg.OldPrice * 100
	}

	subject := fmt.Sprintf("FareWatch: %s→%s dropped to $%.0f on %s",
		msg.Origin, msg.Destination, msg.NewPrice, msg.Airline)
	body := fmt.Sprintf(`Fare drop detected!

Route: %s → %s on %s
Airline: %s
Was: $%.2f
Now: $%.2f (%.1f%% lower)

Book: %s

— FareWatch
`, msg.Origin, msg.Destination, msg.DepartDate, msg.Airline, msg.OldPrice, msg.NewPrice, dropPct, msg.DeepLink)

	raw := strings.Join([]string{
		"From: " + m.from,
		"To: " + msg.To,
		"Subject: " + subject,
		"MIME-Version: 1.0",
		"Content-Type: text/plain; charset=UTF-8",
		"",
		body,
	}, "\r\n")

	err := m.send([]string{msg.To}, []byte(raw))
	return time.Since(start), err
}

func (m *Mailer) send(to []string, msg []byte) error {
	addr := net.JoinHostPort(m.host, m.port)

	// Port 465 = implicit TLS (SMTPS). Everything else tries plain then STARTTLS.
	if m.port == "465" {
		return m.sendTLS(addr, to, msg)
	}
	return m.sendStartTLS(addr, to, msg)
}

func (m *Mailer) sendTLS(addr string, to []string, msg []byte) error {
	conn, err := tls.Dial("tcp", addr, &tls.Config{ServerName: m.host})
	if err != nil {
		return err
	}
	defer conn.Close()

	client, err := smtp.NewClient(conn, m.host)
	if err != nil {
		return err
	}
	defer client.Close()
	return m.deliver(client, to, msg)
}

func (m *Mailer) sendStartTLS(addr string, to []string, msg []byte) error {
	conn, err := net.DialTimeout("tcp", addr, 15*time.Second)
	if err != nil {
		return err
	}

	client, err := smtp.NewClient(conn, m.host)
	if err != nil {
		_ = conn.Close()
		return err
	}
	defer client.Close()

	if ok, _ := client.Extension("STARTTLS"); ok {
		if err := client.StartTLS(&tls.Config{ServerName: m.host}); err != nil {
			return err
		}
	}
	return m.deliver(client, to, msg)
}

func (m *Mailer) deliver(client *smtp.Client, to []string, msg []byte) error {
	if m.user != "" {
		auth := smtp.PlainAuth("", m.user, m.pass, m.host)
		if ok, _ := client.Extension("AUTH"); ok {
			if err := client.Auth(auth); err != nil {
				return err
			}
		}
	}
	if err := client.Mail(m.from); err != nil {
		return err
	}
	for _, addr := range to {
		if err := client.Rcpt(addr); err != nil {
			return err
		}
	}
	w, err := client.Data()
	if err != nil {
		return err
	}
	if _, err := w.Write(msg); err != nil {
		return err
	}
	if err := w.Close(); err != nil {
		return err
	}
	return client.Quit()
}
