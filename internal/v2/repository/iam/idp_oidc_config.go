package iam

import (
	"context"

	"github.com/caos/zitadel/internal/crypto"
	"github.com/caos/zitadel/internal/eventstore/v2"
	"github.com/caos/zitadel/internal/eventstore/v2/repository"
	"github.com/caos/zitadel/internal/v2/repository/idp/oidc"
)

const (
	IDPOIDCConfigAddedEventType   eventstore.EventType = "iam.idp.oidc.config.added"
	IDPOIDCConfigChangedEventType eventstore.EventType = "iam.idp.oidc.config.changed"
)

type IDPOIDCConfigWriteModel struct {
	oidc.ConfigWriteModel

	iamID       string
	idpConfigID string
}

func NewIDPOIDCConfigWriteModel(iamID, idpConfigID string) *IDPOIDCConfigWriteModel {
	return &IDPOIDCConfigWriteModel{
		iamID:       iamID,
		idpConfigID: idpConfigID,
	}
}

func (wm *IDPOIDCConfigWriteModel) Query() *eventstore.SearchQueryFactory {
	return eventstore.NewSearchQueryFactory(eventstore.ColumnsEvent, AggregateType).
		AggregateIDs(wm.iamID)
}

func (wm *IDPOIDCConfigWriteModel) AppendEvents(events ...eventstore.EventReader) {
	for _, event := range events {
		switch e := event.(type) {
		case *IDPOIDCConfigAddedEvent:
			if wm.idpConfigID != e.IDPConfigID {
				continue
			}
			wm.ConfigWriteModel.AppendEvents(&e.ConfigAddedEvent)
		case *IDPOIDCConfigChangedEvent:
			if wm.idpConfigID != e.IDPConfigID {
				continue
			}
			wm.ConfigWriteModel.AppendEvents(&e.ConfigChangedEvent)
		default:
			wm.ConfigWriteModel.AppendEvents(e)
		}
	}
}

type IDPOIDCConfigAddedEvent struct {
	oidc.ConfigAddedEvent
}

func NewIDPOIDCConfigAddedEvent(
	ctx context.Context,
	clientID,
	idpConfigID,
	issuer string,
	clientSecret *crypto.CryptoValue,
	idpDisplayNameMapping,
	userNameMapping oidc.MappingField,
	scopes ...string,
) *IDPOIDCConfigAddedEvent {

	return &IDPOIDCConfigAddedEvent{
		ConfigAddedEvent: *oidc.NewConfigAddedEvent(
			eventstore.NewBaseEventForPush(
				ctx,
				IDPOIDCConfigAddedEventType,
			),
			clientID,
			idpConfigID,
			issuer,
			clientSecret,
			idpDisplayNameMapping,
			userNameMapping,
			scopes...,
		),
	}
}

func IDPOIDCConfigAddedEventMapper(event *repository.Event) (eventstore.EventReader, error) {
	e, err := oidc.ConfigAddedEventMapper(event)
	if err != nil {
		return nil, err
	}

	return &IDPOIDCConfigAddedEvent{ConfigAddedEvent: *e}, nil
}

type IDPOIDCConfigChangedEvent struct {
	oidc.ConfigChangedEvent
}

func NewIDPOIDCConfigChangedEvent(
	ctx context.Context,
	current *IDPOIDCConfigWriteModel,
	clientID,
	issuer string,
	clientSecret *crypto.CryptoValue,
	idpDisplayNameMapping,
	userNameMapping oidc.MappingField,
	scopes ...string,
) (*IDPOIDCConfigChangedEvent, error) {

	event, err := oidc.NewConfigChangedEvent(
		eventstore.NewBaseEventForPush(
			ctx,
			IDPOIDCConfigChangedEventType,
		),
		&current.ConfigWriteModel,
		clientID,
		issuer,
		clientSecret,
		idpDisplayNameMapping,
		userNameMapping,
		scopes...,
	)

	if err != nil {
		return nil, err
	}

	return &IDPOIDCConfigChangedEvent{
		ConfigChangedEvent: *event,
	}, nil
}

func IDPOIDCConfigChangedEventMapper(event *repository.Event) (eventstore.EventReader, error) {
	e, err := oidc.ConfigChangedEventMapper(event)
	if err != nil {
		return nil, err
	}

	return &IDPOIDCConfigChangedEvent{ConfigChangedEvent: *e}, nil
}
