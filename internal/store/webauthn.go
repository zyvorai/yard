package store

import (
	"context"

	"github.com/zyvorai/yard/internal/idgen"
)

func (s *Store) IssueWebAuthnChallenge(ctx context.Context, userID, purpose string) (string, error) {
	id := idgen.New("wch")
	challenge := idgen.New("chal")
	_, err := s.exec(ctx, `INSERT INTO webauthn_challenges(id,user_id,challenge,purpose,created_at) VALUES(?,?,?,?,?)`,
		id, userID, challenge, purpose, now())
	return challenge, err
}

func (s *Store) ConsumeWebAuthnChallenge(ctx context.Context, userID, challenge, purpose string) (bool, error) {
	res, err := s.exec(ctx, `DELETE FROM webauthn_challenges WHERE user_id=? AND challenge=? AND purpose=?`, userID, challenge, purpose)
	if err != nil {
		return false, err
	}
	n, _ := res.RowsAffected()
	return n == 1, nil
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
