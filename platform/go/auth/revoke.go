package auth

func RevokeSession(ctx context.Context, db *sql.DB, rawToken string) error {
    hash := auth.HashRefreshToken(rawToken)

	query := `UPDATE sessions SET revoked_at = NOW() WHERE token_hash = $1 AND revoked_at IS NULL`
    result, err := db.ExecContext(ctx, query, hash)
    if err != nil {
        return err
    }
    return nil
}

func RevokeAllUserSessions(ctx context.Context, db *sql.DB, userID int) error {
    query := `UPDATE sessions SET revoked_at = NOW() WHERE user_id = $1 AND revoked_at IS NULL`
    _, err := db.ExecContext(ctx, query, userID)
    return err
}
