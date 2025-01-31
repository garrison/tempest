package servermain

import (
	"context"
	"errors"

	"capnproto.org/go/capnp/v3"
	"zenhack.net/go/tempest/capnp/collection"
	"zenhack.net/go/tempest/capnp/external"
	"zenhack.net/go/tempest/go/internal/database"
	"zenhack.net/go/tempest/go/internal/server/session"
	"zenhack.net/go/util/exn"
)

var ErrNotLoggedIn = errors.New("You are not logged in.")

type externalApiImpl struct {
	db           database.DB
	userSession  session.UserSession
	sessionStore session.Store
}

func (api externalApiImpl) GetLoginSession(ctx context.Context, p external.ExternalApi_getLoginSession) error {
	if api.userSession.Credential.Type == "" {
		return ErrNotLoggedIn
	}
	results, err := p.AllocResults()
	if err != nil {
		return err
	}
	results.SetSession(external.LoginSession_ServerToClient(loginSessionImpl{
		db:           api.db,
		userSession:  api.userSession,
		sessionStore: api.sessionStore,
	}))
	return nil
}

func (api externalApiImpl) Restore(ctx context.Context, p external.ExternalApi_restore) error {
	return capnp.Unimplemented("ExternalApi.restore() is unimplemented.")
}

type loginSessionImpl struct {
	db           database.DB
	userSession  session.UserSession
	sessionStore session.Store
}

func (loginSessionImpl) UserInfo(context.Context, external.LoginSession_userInfo) error {
	return capnp.Unimplemented("userInfo() not implemented")
}

func (s loginSessionImpl) ListGrains(ctx context.Context, p external.LoginSession_listGrains) error {
	into := p.Args().Into()
	p.Go()
	return exn.Try0(func(throw func(error)) {
		// TODO(cleanup): update our wrapper to support one-off queries without having to
		// create a whole transaction; this is too much boilerplate.
		tx, err := s.db.Begin()
		throw(err)
		defer tx.Rollback()
		c := s.userSession.Credential
		info, err := tx.GetCredentialGrains(c.Type, c.ScopedId)
		throw(err)
		throw(tx.Commit())

		_, rel := into.Clear(ctx, nil)
		releaseFuncs := []capnp.ReleaseFunc{rel}
		for _, grainInfo := range info {
			_, rel = into.Upsert(ctx, func(p collection.Pusher_upsert_Params) error {
				key, err := capnp.NewText(p.Segment(), grainInfo.Id)
				throw(err)
				p.SetKey(key.ToPtr())
				g, err := external.NewGrain(p.Segment())
				throw(err)
				g.SetTitle(grainInfo.Title)
				sessionToken, err := session.GrainSession{
					GrainId:   grainInfo.Id,
					SessionId: s.userSession.SessionId,
				}.Seal(s.sessionStore)
				throw(err)
				g.SetSessionToken(sessionToken)
				// TODO: handle
				p.SetValue(g.ToPtr())
				return nil
			})
			releaseFuncs = append(releaseFuncs, rel)
		}
		_, rel = into.Ready(ctx, nil)
		releaseFuncs = append(releaseFuncs, rel)
		for _, rel := range releaseFuncs {
			rel()
		}
	})
}
