package session

import (
	"context"
	"sort"

	"github.com/xiaoshicae/go-easy-extension/core/errors"
)

const ExtensionDefaultSession = "EXTENSION_DEFAULT_SESSION"

func NewCtxWithSession(ctx context.Context) context.Context {
	return context.WithValue(ctx, ExtensionDefaultSession, &DefaultSession{})
}

type DefaultSession struct {
	codeWithPriorityList []CodeWithPriority
}

func (d *DefaultSession) AddMatchedCode(ctx context.Context, code string, priority int64) error {
	session, ok := ctx.Value(ExtensionDefaultSession).(*DefaultSession)
	if !ok {
		return errors.DefaultSessionNotInjectErr
	}
	session.codeWithPriorityList = append(session.codeWithPriorityList, CodeWithPriority{
		Code:     code,
		Priority: priority,
	})
	sort.Slice(session.codeWithPriorityList, func(i, j int) bool {
		return session.codeWithPriorityList[i].Priority < session.codeWithPriorityList[j].Priority
	})
	return nil
}

func (d *DefaultSession) GetMatchedCodes(ctx context.Context) ([]string, error) {
	session, ok := ctx.Value(ExtensionDefaultSession).(*DefaultSession)
	if !ok {
		return nil, errors.DefaultSessionNotInjectErr
	}
	matchedCodes := make([]string, 0)
	for _, codeWithPriority := range session.codeWithPriorityList {
		matchedCodes = append(matchedCodes, codeWithPriority.Code)
	}
	return matchedCodes, nil
}

func (d *DefaultSession) Remove(ctx context.Context) {
	session, ok := ctx.Value(ExtensionDefaultSession).(*DefaultSession)
	if !ok {
		return
	}
	session.codeWithPriorityList = make([]CodeWithPriority, 0)
}

type CodeWithPriority struct {
	Code     string
	Priority int64
}
