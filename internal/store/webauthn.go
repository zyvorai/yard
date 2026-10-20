package store

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/zyvorai/yard/internal/idgen"
)

// webauthnChallengeTTL bounds how long an issued challenge stays valid.
const webauthnChallengeTTL = 5 * time.Minute

func (s *Store) IssueWebAuthnChallenge(ctx context.Context, userID, purpose string) (string, error) {
	id := idgen.New("wch")
	challenge := idgen.Secret(32)
	_, err := s.exec(ctx, `INSERT INTO webauthn_challenges(id,user_id,challenge,purpose,created_at) VALUES(?,?,?,?,?)`,
		id, userID, challenge, purpose, now())
	return challenge, err
}

func (s *Store) ConsumeWebAuthnChallenge(ctx context.Context, userID, challenge, purpose string) (bool, error) {
	var created string
	err := s.queryRow(ctx, `SELECT created_at FROM webauthn_challenges WHERE user_id=? AND challenge=? AND purpose=?`, userID, challenge, purpose).Scan(&created)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	res, err := s.exec(ctx, `DELETE FROM webauthn_challenges WHERE user_id=? AND challenge=? AND purpose=?`, userID, challenge, purpose)
	if err != nil {
		return false, err
	}
	n, _ := res.RowsAffected()
	if n != 1 {
		return false, nil
	}
	return time.Since(parseTime(created)) <= webauthnChallengeTTL, nil
}

func (s *Store) SaveWebAuthnCredential(ctx context.Context, orgID, userID, credentialID, publicKey string) error {
	_, err := s.exec(ctx, `INSERT INTO webauthn_credentials(id,organization_id,user_id,credential_id,public_key,created_at) VALUES(?,?,?,?,?,?)`,
		idgen.New("wcr"), orgID, userID, credentialID, publicKey, now())
	return err
}

func (s *Store) WebAuthnCredential(ctx context.Context, userID, credentialID string) (bool, error) {
	var n int
	err := s.queryRow(ctx, `SELECT COUNT(*) FROM webauthn_credentials WHERE user_id=? AND credential_id=?`, userID, credentialID).Scan(&n)
	return n > 0, err
}
