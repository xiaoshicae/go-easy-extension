package session

import "context"

type ISession interface {
	AddMatchedCode(ctx context.Context, code string, priority int64) error
	GetMatchedCodes(ctx context.Context) ([]string, error)
	Remove(ctx context.Context)
}
