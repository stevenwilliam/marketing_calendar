// Package notify sends outbound messages. Email today; the Sender port is what
// WhatsApp will implement (D16), so swapping the provider is an adapter change
// and never a reshape of the approval flow.
package notify

import (
	"context"
	"fmt"
	"log/slog"
	"net/smtp"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/stevenwilliam/marketing_calendar/internal/platform/config"
	"github.com/stevenwilliam/marketing_calendar/internal/platform/id"
	"github.com/stevenwilliam/marketing_calendar/internal/platform/sanitize"
	"gorm.io/gorm"
)

type SMTP struct {
	cfg config.Config
	db  *gorm.DB
	log *slog.Logger
}

func NewSMTP(cfg config.Config, db *gorm.DB, log *slog.Logger) *SMTP {
	return &SMTP{cfg: cfg, db: db, log: log}
}

// Queue writes the message and returns. It never dials SMTP.
//
// This is the whole point: an approval decision has already committed by the
// time this runs, and a mail server that is down must not turn a successful
// approval into a 500 that the approver retries — producing a second decision
// attempt against a chain that already moved.
func (s *SMTP) Queue(ctx context.Context, recipients []string, subject, body string, subjectType string, subjectID *uuid.UUID) error {
	if len(recipients) == 0 {
		return nil
	}
	var sid any
	if subjectID != nil {
		sid = *subjectID
	}
	for _, to := range recipients {
		clean, err := sanitize.Email(to)
		if err != nil {
			// A malformed address in a maintained list is worth a log line;
			// it must not stop the other recipients.
			s.log.Warn("notification recipient rejected",
				slog.String("recipient", sanitize.LogValue(to)), slog.String("err", err.Error()))
			continue
		}
		err = s.db.WithContext(ctx).Exec(`
			INSERT INTO notification_log (notification_id, channel, recipient, subject, body,
			    subject_type, subject_id, status)
			VALUES (?, 'email', ?, ?, ?, ?, ?, 'QUEUED')`,
			id.New(), clean, subject, body, subjectType, sid).Error
		if err != nil {
			return err
		}
	}
	return nil
}

// Flush attempts delivery. A failure increments attempts and records the
// error; it does not lose the message.
func (s *SMTP) Flush(ctx context.Context, limit int) (int, int, error) {
	if !s.cfg.NotifyEnabled {
		return 0, 0, nil
	}
	if limit <= 0 {
		limit = 100
	}
	rows, err := s.db.WithContext(ctx).Raw(`
		SELECT notification_id, recipient, subject, body
		  FROM notification_log
		 WHERE status = 'QUEUED' AND attempts < 5
		 ORDER BY created_at LIMIT ?`, limit).Rows()
	if err != nil {
		return 0, 0, err
	}
	type item struct {
		id                uuid.UUID
		to, subject, body string
	}
	var batch []item
	for rows.Next() {
		var it item
		if err := rows.Scan(&it.id, &it.to, &it.subject, &it.body); err != nil {
			rows.Close()
			return 0, 0, err
		}
		batch = append(batch, it)
	}
	rows.Close()

	var sent, failed int
	for _, it := range batch {
		if err := s.send(it.to, it.subject, it.body); err != nil {
			failed++
			_ = s.db.WithContext(ctx).Exec(`
				UPDATE notification_log
				   SET attempts = attempts + 1, last_error = ?,
				       status = CASE WHEN attempts + 1 >= 5 THEN 'FAILED' ELSE 'QUEUED' END
				 WHERE notification_id = ?`, err.Error(), it.id).Error
			continue
		}
		sent++
		_ = s.db.WithContext(ctx).Exec(`
			UPDATE notification_log SET status = 'SENT', sent_at = now(),
			       attempts = attempts + 1 WHERE notification_id = ?`, it.id).Error
	}
	return sent, failed, nil
}

func (s *SMTP) send(to, subject, body string) error {
	addr := fmt.Sprintf("%s:%d", s.cfg.SMTPHost, s.cfg.SMTPPort)
	// Header injection: a newline in a subject forges headers. The subject is
	// built by this application, but building the guard here means it holds
	// even when somebody later renders a promo name into it.
	subject = strings.NewReplacer("\r", " ", "\n", " ").Replace(subject)
	msg := strings.Join([]string{
		"From: " + s.cfg.SMTPFrom,
		"To: " + to,
		"Subject: " + subject,
		"Date: " + time.Now().Format(time.RFC1123Z),
		"MIME-Version: 1.0",
		"Content-Type: text/plain; charset=UTF-8",
		"",
		body,
	}, "\r\n")
	return smtp.SendMail(addr, nil, s.cfg.SMTPFrom, []string{to}, []byte(msg))
}
