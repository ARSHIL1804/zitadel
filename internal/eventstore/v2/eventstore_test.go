package eventstore

import (
	"context"
	"fmt"
	"reflect"
	"sync"
	"testing"

	"github.com/caos/zitadel/internal/errors"
	"github.com/caos/zitadel/internal/eventstore/v2/repository"
)

type testAggregate struct {
	id               string
	events           []Event
	previousSequence uint64
}

func (a *testAggregate) ID() string {
	return a.id
}

func (a *testAggregate) Type() AggregateType {
	return "test.aggregate"
}

func (a *testAggregate) Events() []Event {
	return a.events
}

func (a *testAggregate) ResourceOwner() string {
	return "ro"
}

func (a *testAggregate) Version() Version {
	return "v1"
}

func (a *testAggregate) PreviousSequence() uint64 {
	return a.previousSequence
}

// testEvent implements the Event interface
type testEvent struct {
	description         string
	shouldCheckPrevious bool
	data                func() interface{}
}

func (e *testEvent) CheckPrevious() bool {
	return e.shouldCheckPrevious
}

func (e *testEvent) EditorService() string {
	return "editorService"
}
func (e *testEvent) EditorUser() string {
	return "editorUser"
}
func (e *testEvent) Type() EventType {
	return "test.event"
}
func (e *testEvent) Data() interface{} {
	return e.data()
}

func (e *testEvent) PreviousSequence() uint64 {
	return 0
}

func (e *testEvent) MetaData() *EventMetaData {
	return nil
}

func testFilterMapper(*repository.Event) (Event, error) {
	return &testEvent{description: "hodor"}, nil
}

func Test_eventstore_RegisterFilterEventMapper(t *testing.T) {
	type fields struct {
		eventMapper map[EventType]eventTypeInterceptors
	}
	type args struct {
		eventType EventType
		mapper    func(*repository.Event) (Event, error)
	}
	type res struct {
		event       Event
		mapperCount int
	}

	tests := []struct {
		name   string
		fields fields
		args   args
		res    res
	}{
		{
			name: "no event type",
			args: args{
				eventType: "",
				mapper:    testFilterMapper,
			},
			fields: fields{
				eventMapper: map[EventType]eventTypeInterceptors{},
			},
			res: res{
				mapperCount: 0,
			},
		},
		{
			name: "no event mapper",
			args: args{
				eventType: "event.type",
				mapper:    nil,
			},
			fields: fields{
				eventMapper: map[EventType]eventTypeInterceptors{},
			},
			res: res{
				mapperCount: 0,
			},
		},
		{
			name: "new interceptor",
			fields: fields{
				eventMapper: map[EventType]eventTypeInterceptors{},
			},
			args: args{
				eventType: "event.type",
				mapper:    testFilterMapper,
			},
			res: res{
				event:       &testEvent{description: "hodor"},
				mapperCount: 1,
			},
		},
		{
			name: "existing interceptor new filter mapper",
			fields: fields{
				eventMapper: map[EventType]eventTypeInterceptors{
					"event.type": {},
				},
			},
			args: args{
				eventType: "new.event",
				mapper:    testFilterMapper,
			},
			res: res{
				event:       &testEvent{description: "hodor"},
				mapperCount: 2,
			},
		},
		{
			name: "existing interceptor existing filter mapper",
			fields: fields{
				eventMapper: map[EventType]eventTypeInterceptors{
					"event.type": {
						eventMapper: func(*repository.Event) (Event, error) {
							return nil, errors.ThrowUnimplemented(nil, "V2-1qPvn", "unimplemented")
						},
					},
				},
			},
			args: args{
				eventType: "new.event",
				mapper:    testFilterMapper,
			},
			res: res{
				event:       &testEvent{description: "hodor"},
				mapperCount: 2,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			es := &Eventstore{
				eventMapper: tt.fields.eventMapper,
			}
			es = es.RegisterFilterEventMapper(tt.args.eventType, tt.args.mapper)
			if len(es.eventMapper) != tt.res.mapperCount {
				t.Errorf("unexpected mapper count: want %d, got %d", tt.res.mapperCount, len(es.eventMapper))
				return
			}

			if tt.res.mapperCount == 0 {
				return
			}

			mapper := es.eventMapper[tt.args.eventType]
			event, err := mapper.eventMapper(nil)
			if err != nil {
				t.Errorf("unexpected error %v", err)
			}

			if !reflect.DeepEqual(tt.res.event, event) {
				t.Errorf("events should be deep equal. \ngot %v\nwant %v", event, tt.res.event)
			}
		})
	}
}

func Test_eventData(t *testing.T) {
	type args struct {
		event Event
	}
	type res struct {
		jsonText []byte
		wantErr  bool
	}
	tests := []struct {
		name string
		args args
		res  res
	}{
		{
			name: "data as json bytes",
			args: args{
				event: &testEvent{
					data: func() interface{} {
						return []byte(`{"piff":"paff"}`)
					},
				},
			},
			res: res{
				jsonText: []byte(`{"piff":"paff"}`),
				wantErr:  false,
			},
		},
		{
			name: "data as invalid json bytes",
			args: args{
				event: &testEvent{
					data: func() interface{} {
						return []byte(`{"piffpaff"}`)
					},
				},
			},
			res: res{
				jsonText: []byte(nil),
				wantErr:  true,
			},
		},
		{
			name: "data as struct",
			args: args{
				event: &testEvent{
					data: func() interface{} {
						return struct {
							Piff string `json:"piff"`
						}{Piff: "paff"}
					},
				},
			},
			res: res{
				jsonText: []byte(`{"piff":"paff"}`),
				wantErr:  false,
			},
		},
		{
			name: "data as ptr to struct",
			args: args{
				event: &testEvent{
					data: func() interface{} {
						return &struct {
							Piff string `json:"piff"`
						}{Piff: "paff"}
					},
				},
			},
			res: res{
				jsonText: []byte(`{"piff":"paff"}`),
				wantErr:  false,
			},
		},
		{
			name: "no data",
			args: args{
				event: &testEvent{
					data: func() interface{} {
						return nil
					},
				},
			},
			res: res{
				jsonText: []byte(nil),
				wantErr:  false,
			},
		},
		{
			name: "invalid because primitive",
			args: args{
				event: &testEvent{
					data: func() interface{} {
						return ""
					},
				},
			},
			res: res{
				jsonText: []byte(nil),
				wantErr:  true,
			},
		},
		{
			name: "invalid because pointer to primitive",
			args: args{
				event: &testEvent{
					data: func() interface{} {
						var s string
						return &s
					},
				},
			},
			res: res{
				jsonText: []byte(nil),
				wantErr:  true,
			},
		},
		{
			name: "invalid because invalid struct for json",
			args: args{
				event: &testEvent{
					data: func() interface{} {
						return struct {
							Field chan string `json:"field"`
						}{}
					},
				},
			},
			res: res{
				jsonText: []byte(nil),
				wantErr:  true,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := eventData(tt.args.event)
			if (err != nil) != tt.res.wantErr {
				t.Errorf("eventData() error = %v, wantErr %v", err, tt.res.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.res.jsonText) {
				t.Errorf("eventData() = %v, want %v", string(got), string(tt.res.jsonText))
			}
		})
	}
}

func TestEventstore_aggregatesToEvents(t *testing.T) {
	type args struct {
		aggregates []aggregater
	}
	type res struct {
		wantErr bool
		events  []*repository.Event
	}
	tests := []struct {
		name string
		args args
		res  res
	}{
		{
			name: "one aggregate one event",
			args: args{
				aggregates: []aggregater{
					&testAggregate{
						id: "1",
						events: []Event{
							&testEvent{
								data:                func() interface{} { return nil },
								shouldCheckPrevious: false,
							},
						},
					},
				},
			},
			res: res{
				wantErr: false,
				events: []*repository.Event{
					{
						AggregateID:           "1",
						AggregateType:         "test.aggregate",
						CheckPreviousSequence: false,
						Data:                  []byte(nil),
						EditorService:         "editorService",
						EditorUser:            "editorUser",
						ResourceOwner:         "ro",
						Type:                  "test.event",
						Version:               "v1",
					},
				},
			},
		},
		{
			name: "one aggregate multiple events",
			args: args{
				aggregates: []aggregater{
					&testAggregate{
						id: "1",
						events: []Event{
							&testEvent{
								data:                func() interface{} { return nil },
								shouldCheckPrevious: false,
							},
							&testEvent{
								data:                func() interface{} { return nil },
								shouldCheckPrevious: false,
							},
						},
					},
				},
			},
			res: res{
				wantErr: false,
				events: linkEvents(
					&repository.Event{
						AggregateID:           "1",
						AggregateType:         "test.aggregate",
						CheckPreviousSequence: false,
						Data:                  []byte(nil),
						EditorService:         "editorService",
						EditorUser:            "editorUser",
						ResourceOwner:         "ro",
						Type:                  "test.event",
						Version:               "v1",
					},
					&repository.Event{
						AggregateID:           "1",
						AggregateType:         "test.aggregate",
						CheckPreviousSequence: false,
						Data:                  []byte(nil),
						EditorService:         "editorService",
						EditorUser:            "editorUser",
						ResourceOwner:         "ro",
						Type:                  "test.event",
						Version:               "v1",
					},
				),
			},
		},
		{
			name: "invalid data",
			args: args{
				aggregates: []aggregater{
					&testAggregate{
						id: "1",
						events: []Event{
							&testEvent{
								data:                func() interface{} { return `{"data":""` },
								shouldCheckPrevious: false,
							},
						},
					},
				},
			},
			res: res{
				wantErr: true,
			},
		},
		{
			name: "multiple aggregates",
			args: args{
				aggregates: []aggregater{
					&testAggregate{
						id: "1",
						events: []Event{
							&testEvent{
								data:                func() interface{} { return nil },
								shouldCheckPrevious: false,
							},
							&testEvent{
								data:                func() interface{} { return nil },
								shouldCheckPrevious: false,
							},
						},
					},
					&testAggregate{
						id: "2",
						events: []Event{
							&testEvent{
								data:                func() interface{} { return nil },
								shouldCheckPrevious: true,
							},
						},
					},
				},
			},
			res: res{
				wantErr: false,
				events: combineEventLists(
					linkEvents(
						&repository.Event{
							AggregateID:           "1",
							AggregateType:         "test.aggregate",
							CheckPreviousSequence: false,
							Data:                  []byte(nil),
							EditorService:         "editorService",
							EditorUser:            "editorUser",
							ResourceOwner:         "ro",
							Type:                  "test.event",
							Version:               "v1",
						},
						&repository.Event{
							AggregateID:           "1",
							AggregateType:         "test.aggregate",
							CheckPreviousSequence: false,
							Data:                  []byte(nil),
							EditorService:         "editorService",
							EditorUser:            "editorUser",
							ResourceOwner:         "ro",
							Type:                  "test.event",
							Version:               "v1",
						},
					),
					[]*repository.Event{
						{
							AggregateID:           "2",
							AggregateType:         "test.aggregate",
							CheckPreviousSequence: true,
							Data:                  []byte(nil),
							EditorService:         "editorService",
							EditorUser:            "editorUser",
							ResourceOwner:         "ro",
							Type:                  "test.event",
							Version:               "v1",
						},
					},
				),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			es := &Eventstore{}
			events, err := es.aggregatesToEvents(tt.args.aggregates)
			if (err != nil) != tt.res.wantErr {
				t.Errorf("Eventstore.aggregatesToEvents() error = %v, wantErr %v", err, tt.res.wantErr)
				return
			}

			if err != nil {
				return
			}

			if len(tt.res.events) != len(events) {
				t.Errorf("length of events unequal want: %d got %d", len(tt.res.events), len(events))
				return
			}

			for i := 0; i < len(tt.res.events); i++ {
				compareEvents(t, tt.res.events[i], events[i])
			}
		})
	}
}

type testRepo struct {
	events   []*repository.Event
	sequence uint64
	err      error
	t        *testing.T
}

func (repo *testRepo) Health(ctx context.Context) error {
	return nil
}

func (repo *testRepo) Push(ctx context.Context, events ...*repository.Event) error {
	if repo.err != nil {
		return repo.err
	}

	if len(repo.events) != len(events) {
		repo.t.Errorf("length of events unequal want: %d got %d", len(repo.events), len(events))
		return fmt.Errorf("")
	}

	for i := 0; i < len(repo.events); i++ {
		compareEvents(repo.t, repo.events[i], events[i])
	}

	return nil
}

func (repo *testRepo) Filter(ctx context.Context, searchQuery *repository.SearchQuery) ([]*repository.Event, error) {
	if repo.err != nil {
		return nil, repo.err
	}
	return repo.events, nil
}

func (repo *testRepo) LatestSequence(ctx context.Context, queryFactory *repository.SearchQuery) (uint64, error) {
	if repo.err != nil {
		return 0, repo.err
	}
	return repo.sequence, nil
}

func TestEventstore_Push(t *testing.T) {
	type args struct {
		aggregates []aggregater
	}
	type fields struct {
		repo        *testRepo
		eventMapper map[EventType]func(*repository.Event) (Event, error)
	}
	type res struct {
		wantErr bool
	}
	tests := []struct {
		name   string
		args   args
		fields fields
		res    res
	}{
		{
			name: "one aggregate one event",
			args: args{
				aggregates: []aggregater{
					&testAggregate{
						id: "1",
						events: []Event{
							&testEvent{
								data:                func() interface{} { return nil },
								shouldCheckPrevious: false,
							},
						},
					},
				},
			},
			fields: fields{
				repo: &testRepo{
					t: t,
					events: []*repository.Event{
						{
							AggregateID:           "1",
							AggregateType:         "test.aggregate",
							CheckPreviousSequence: false,
							Data:                  []byte(nil),
							EditorService:         "editorService",
							EditorUser:            "editorUser",
							ResourceOwner:         "ro",
							Type:                  "test.event",
							Version:               "v1",
						},
					},
				},
				eventMapper: map[EventType]func(*repository.Event) (Event, error){
					"test.event": func(e *repository.Event) (Event, error) {
						return &testEvent{}, nil
					},
				},
			},
		},
		{
			name: "one aggregate multiple events",
			args: args{
				aggregates: []aggregater{
					&testAggregate{
						id: "1",
						events: []Event{
							&testEvent{
								data:                func() interface{} { return nil },
								shouldCheckPrevious: false,
							},
							&testEvent{
								data:                func() interface{} { return nil },
								shouldCheckPrevious: false,
							},
						},
					},
				},
			},
			fields: fields{
				repo: &testRepo{
					t: t,
					events: linkEvents(
						&repository.Event{
							AggregateID:           "1",
							AggregateType:         "test.aggregate",
							CheckPreviousSequence: false,
							Data:                  []byte(nil),
							EditorService:         "editorService",
							EditorUser:            "editorUser",
							ResourceOwner:         "ro",
							Type:                  "test.event",
							Version:               "v1",
						},
						&repository.Event{
							AggregateID:           "1",
							AggregateType:         "test.aggregate",
							CheckPreviousSequence: false,
							Data:                  []byte(nil),
							EditorService:         "editorService",
							EditorUser:            "editorUser",
							ResourceOwner:         "ro",
							Type:                  "test.event",
							Version:               "v1",
						},
					),
				},
				eventMapper: map[EventType]func(*repository.Event) (Event, error){
					"test.event": func(e *repository.Event) (Event, error) {
						return &testEvent{}, nil
					},
				},
			},
			res: res{
				wantErr: false,
			},
		},
		{
			name: "multiple aggregates",
			args: args{
				aggregates: []aggregater{
					&testAggregate{
						id: "1",
						events: []Event{
							&testEvent{
								data:                func() interface{} { return nil },
								shouldCheckPrevious: false,
							},
							&testEvent{
								data:                func() interface{} { return nil },
								shouldCheckPrevious: false,
							},
						},
					},
					&testAggregate{
						id: "2",
						events: []Event{
							&testEvent{
								data:                func() interface{} { return nil },
								shouldCheckPrevious: true,
							},
						},
					},
				},
			},
			fields: fields{
				repo: &testRepo{
					t: t,
					events: combineEventLists(
						linkEvents(
							&repository.Event{
								AggregateID:           "1",
								AggregateType:         "test.aggregate",
								CheckPreviousSequence: false,
								Data:                  []byte(nil),
								EditorService:         "editorService",
								EditorUser:            "editorUser",
								ResourceOwner:         "ro",
								Type:                  "test.event",
								Version:               "v1",
							},
							&repository.Event{
								AggregateID:           "1",
								AggregateType:         "test.aggregate",
								CheckPreviousSequence: false,
								Data:                  []byte(nil),
								EditorService:         "editorService",
								EditorUser:            "editorUser",
								ResourceOwner:         "ro",
								Type:                  "test.event",
								Version:               "v1",
							},
						),
						[]*repository.Event{
							{
								AggregateID:           "2",
								AggregateType:         "test.aggregate",
								CheckPreviousSequence: true,
								Data:                  []byte(nil),
								EditorService:         "editorService",
								EditorUser:            "editorUser",
								ResourceOwner:         "ro",
								Type:                  "test.event",
								Version:               "v1",
							},
						},
					),
				},
				eventMapper: map[EventType]func(*repository.Event) (Event, error){
					"test.event": func(e *repository.Event) (Event, error) {
						return &testEvent{}, nil
					},
				},
			},
			res: res{
				wantErr: false,
			},
		},
		{
			name: "push fails",
			args: args{
				aggregates: []aggregater{
					&testAggregate{
						id: "1",
						events: []Event{
							&testEvent{
								data:                func() interface{} { return nil },
								shouldCheckPrevious: false,
							},
						},
					},
				},
			},
			fields: fields{
				repo: &testRepo{
					t:   t,
					err: errors.ThrowInternal(nil, "V2-qaa4S", "test err"),
				},
			},
			res: res{
				wantErr: true,
			},
		},
		{
			name: "aggreagtes to events mapping fails",
			args: args{
				aggregates: []aggregater{
					&testAggregate{
						id: "1",
						events: []Event{
							&testEvent{
								data:                func() interface{} { return `{"data":""` },
								shouldCheckPrevious: false,
							},
						},
					},
				},
			},
			fields: fields{
				repo: &testRepo{
					t:   t,
					err: errors.ThrowInternal(nil, "V2-qaa4S", "test err"),
				},
			},
			res: res{
				wantErr: true,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			es := &Eventstore{
				repo:             tt.fields.repo,
				interceptorMutex: sync.Mutex{},
				eventMapper:      map[EventType]eventTypeInterceptors{},
			}
			for eventType, mapper := range tt.fields.eventMapper {
				es = es.RegisterFilterEventMapper(eventType, mapper)
			}
			if len(es.eventMapper) != len(tt.fields.eventMapper) {
				t.Errorf("register event mapper failed expected mapper amount: %d, got: %d", len(tt.fields.eventMapper), len(es.eventMapper))
				t.FailNow()
			}

			_, err := es.PushAggregates(context.Background(), tt.args.aggregates...)
			if (err != nil) != tt.res.wantErr {
				t.Errorf("Eventstore.aggregatesToEvents() error = %v, wantErr %v", err, tt.res.wantErr)
			}
		})
	}
}

func TestEventstore_FilterEvents(t *testing.T) {
	type args struct {
		query *SearchQueryFactory
	}
	type fields struct {
		repo        *testRepo
		eventMapper map[EventType]func(*repository.Event) (Event, error)
	}
	type res struct {
		wantErr bool
	}
	tests := []struct {
		name   string
		args   args
		fields fields
		res    res
	}{
		{
			name: "invalid factory",
			args: args{
				query: nil,
			},
			res: res{
				wantErr: true,
			},
		},
		{
			name: "no events",
			args: args{
				query: &SearchQueryFactory{
					aggregateTypes: []AggregateType{"no.aggregates"},
					columns:        repository.ColumnsEvent,
				},
			},
			fields: fields{
				repo: &testRepo{
					events: []*repository.Event{},
					t:      t,
				},
				eventMapper: map[EventType]func(*repository.Event) (Event, error){
					"test.event": func(e *repository.Event) (Event, error) {
						return &testEvent{}, nil
					},
				},
			},
			res: res{
				wantErr: false,
			},
		},
		{
			name: "repo error",
			args: args{
				query: &SearchQueryFactory{
					aggregateTypes: []AggregateType{"no.aggregates"},
					columns:        repository.ColumnsEvent,
				},
			},
			fields: fields{
				repo: &testRepo{
					t:   t,
					err: errors.ThrowInternal(nil, "V2-RfkBa", "test err"),
				},
				eventMapper: map[EventType]func(*repository.Event) (Event, error){
					"test.event": func(e *repository.Event) (Event, error) {
						return &testEvent{}, nil
					},
				},
			},
			res: res{
				wantErr: true,
			},
		},
		{
			name: "found events",
			args: args{
				query: &SearchQueryFactory{
					aggregateTypes: []AggregateType{"test.aggregate"},
					columns:        repository.ColumnsEvent,
				},
			},
			fields: fields{
				repo: &testRepo{
					events: []*repository.Event{
						{
							AggregateID: "test.aggregate",
							Type:        "test.event",
						},
					},
					t: t,
				},
				eventMapper: map[EventType]func(*repository.Event) (Event, error){
					"test.event": func(e *repository.Event) (Event, error) {
						return &testEvent{}, nil
					},
				},
			},
			res: res{
				wantErr: false,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			es := &Eventstore{
				repo:             tt.fields.repo,
				interceptorMutex: sync.Mutex{},
				eventMapper:      map[EventType]eventTypeInterceptors{},
			}

			for eventType, mapper := range tt.fields.eventMapper {
				es = es.RegisterFilterEventMapper(eventType, mapper)
			}
			if len(es.eventMapper) != len(tt.fields.eventMapper) {
				t.Errorf("register event mapper failed expected mapper amount: %d, got: %d", len(tt.fields.eventMapper), len(es.eventMapper))
				t.FailNow()
			}

			_, err := es.FilterEvents(context.Background(), tt.args.query)
			if (err != nil) != tt.res.wantErr {
				t.Errorf("Eventstore.aggregatesToEvents() error = %v, wantErr %v", err, tt.res.wantErr)
			}
		})
	}
}

func TestEventstore_LatestSequence(t *testing.T) {
	type args struct {
		query *SearchQueryFactory
	}
	type fields struct {
		repo *testRepo
	}
	type res struct {
		wantErr bool
	}
	tests := []struct {
		name   string
		args   args
		fields fields
		res    res
	}{
		{
			name: "invalid factory",
			args: args{
				query: nil,
			},
			res: res{
				wantErr: true,
			},
		},
		{
			name: "no events",
			args: args{
				query: &SearchQueryFactory{
					aggregateTypes: []AggregateType{"no.aggregates"},
					columns:        repository.ColumnsMaxSequence,
				},
			},
			fields: fields{
				repo: &testRepo{
					events: []*repository.Event{},
					t:      t,
				},
			},
			res: res{
				wantErr: false,
			},
		},
		{
			name: "repo error",
			args: args{
				query: &SearchQueryFactory{
					aggregateTypes: []AggregateType{"no.aggregates"},
					columns:        repository.ColumnsMaxSequence,
				},
			},
			fields: fields{
				repo: &testRepo{
					t:   t,
					err: errors.ThrowInternal(nil, "V2-RfkBa", "test err"),
				},
			},
			res: res{
				wantErr: true,
			},
		},
		{
			name: "found events",
			args: args{
				query: &SearchQueryFactory{
					aggregateTypes: []AggregateType{"test.aggregate"},
					columns:        repository.ColumnsMaxSequence,
				},
			},
			fields: fields{
				repo: &testRepo{
					sequence: 50,
					t:        t,
				},
			},
			res: res{
				wantErr: false,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			es := &Eventstore{
				repo: tt.fields.repo,
			}

			_, err := es.LatestSequence(context.Background(), tt.args.query)
			if (err != nil) != tt.res.wantErr {
				t.Errorf("Eventstore.aggregatesToEvents() error = %v, wantErr %v", err, tt.res.wantErr)
			}
		})
	}
}

type testReducer struct {
	t              *testing.T
	events         []Event
	expectedLength int
	err            error
}

func (r *testReducer) Reduce() error {
	r.t.Helper()
	if len(r.events) != r.expectedLength {
		r.t.Errorf("wrong amount of append events wanted: %d, got %d", r.expectedLength, len(r.events))
	}
	if r.err != nil {
		return r.err
	}
	return nil
}

func (r *testReducer) AppendEvents(e ...Event) error {
	if r.err != nil {
		return r.err
	}
	r.events = append(r.events, e...)
	return nil
}

func TestEventstore_FilterToReducer(t *testing.T) {
	type args struct {
		query     *SearchQueryFactory
		readModel reducer
	}
	type fields struct {
		repo        *testRepo
		eventMapper map[EventType]func(*repository.Event) (Event, error)
	}
	type res struct {
		wantErr bool
	}
	tests := []struct {
		name   string
		args   args
		fields fields
		res    res
	}{
		{
			name: "invalid factory",
			args: args{
				query: nil,
			},
			res: res{
				wantErr: true,
			},
		},
		{
			name: "no events",
			args: args{
				query: &SearchQueryFactory{
					aggregateTypes: []AggregateType{"no.aggregates"},
					columns:        repository.ColumnsEvent,
				},
				readModel: &testReducer{
					t:              t,
					expectedLength: 0,
				},
			},
			fields: fields{
				repo: &testRepo{
					events: []*repository.Event{},
					t:      t,
				},
				eventMapper: map[EventType]func(*repository.Event) (Event, error){
					"test.event": func(e *repository.Event) (Event, error) {
						return &testEvent{}, nil
					},
				},
			},
			res: res{
				wantErr: false,
			},
		},
		{
			name: "repo error",
			args: args{
				query: &SearchQueryFactory{
					aggregateTypes: []AggregateType{"no.aggregates"},
					columns:        repository.ColumnsEvent,
				},
				readModel: &testReducer{
					t:              t,
					expectedLength: 0,
				},
			},
			fields: fields{
				repo: &testRepo{
					t:   t,
					err: errors.ThrowInternal(nil, "V2-RfkBa", "test err"),
				},
				eventMapper: map[EventType]func(*repository.Event) (Event, error){
					"test.event": func(e *repository.Event) (Event, error) {
						return &testEvent{}, nil
					},
				},
			},
			res: res{
				wantErr: true,
			},
		},
		{
			name: "found events",
			args: args{
				query: &SearchQueryFactory{
					aggregateTypes: []AggregateType{"test.aggregate"},
					columns:        repository.ColumnsEvent,
				},
				readModel: &testReducer{
					t:              t,
					expectedLength: 1,
				},
			},
			fields: fields{
				repo: &testRepo{
					events: []*repository.Event{
						{
							AggregateID: "test.aggregate",
							Type:        "test.event",
						},
					},
					t: t,
				},
				eventMapper: map[EventType]func(*repository.Event) (Event, error){
					"test.event": func(e *repository.Event) (Event, error) {
						return &testEvent{}, nil
					},
				},
			},
		},
		{
			name: "append in reducer fails",
			args: args{
				query: &SearchQueryFactory{
					aggregateTypes: []AggregateType{"test.aggregate"},
					columns:        repository.ColumnsEvent,
				},
				readModel: &testReducer{
					t:   t,
					err: errors.ThrowInvalidArgument(nil, "V2-W06TG", "test err"),
				},
			},
			fields: fields{
				repo: &testRepo{
					events: []*repository.Event{
						{
							AggregateID: "test.aggregate",
							Type:        "test.event",
						},
					},
					t: t,
				},
				eventMapper: map[EventType]func(*repository.Event) (Event, error){
					"test.event": func(e *repository.Event) (Event, error) {
						return &testEvent{}, nil
					},
				},
			},
			res: res{
				wantErr: true,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			es := &Eventstore{
				repo:             tt.fields.repo,
				interceptorMutex: sync.Mutex{},
				eventMapper:      map[EventType]eventTypeInterceptors{},
			}
			for eventType, mapper := range tt.fields.eventMapper {
				es = es.RegisterFilterEventMapper(eventType, mapper)
			}
			if len(es.eventMapper) != len(tt.fields.eventMapper) {
				t.Errorf("register event mapper failed expected mapper amount: %d, got: %d", len(tt.fields.eventMapper), len(es.eventMapper))
				t.FailNow()
			}

			err := es.FilterToReducer(context.Background(), tt.args.query, tt.args.readModel)
			if (err != nil) != tt.res.wantErr {
				t.Errorf("Eventstore.aggregatesToEvents() error = %v, wantErr %v", err, tt.res.wantErr)
			}
		})
	}
}

func combineEventLists(lists ...[]*repository.Event) []*repository.Event {
	events := []*repository.Event{}
	for _, list := range lists {
		events = append(events, list...)
	}
	return events
}

func linkEvents(events ...*repository.Event) []*repository.Event {
	for i := 1; i < len(events); i++ {
		events[i].PreviousEvent = events[i-1]
	}
	return events
}

func compareEvents(t *testing.T, want, got *repository.Event) {
	t.Helper()

	if want.AggregateID != got.AggregateID {
		t.Errorf("wrong aggregateID got %q want %q", want.AggregateID, got.AggregateID)
	}
	if want.AggregateType != got.AggregateType {
		t.Errorf("wrong aggregateType got %q want %q", want.AggregateType, got.AggregateType)
	}
	if want.CheckPreviousSequence != got.CheckPreviousSequence {
		t.Errorf("wrong check previous got %v want %v", want.CheckPreviousSequence, got.CheckPreviousSequence)
	}
	if !reflect.DeepEqual(want.Data, got.Data) {
		t.Errorf("wrong data got %s want %s", string(want.Data), string(got.Data))
	}
	if want.EditorService != got.EditorService {
		t.Errorf("wrong editor service got %q want %q", got.EditorService, want.EditorService)
	}
	if want.EditorUser != got.EditorUser {
		t.Errorf("wrong editor user got %q want %q", got.EditorUser, want.EditorUser)
	}
	if want.ResourceOwner != got.ResourceOwner {
		t.Errorf("wrong resource owner got %q want %q", got.ResourceOwner, want.ResourceOwner)
	}
	if want.Type != got.Type {
		t.Errorf("wrong event type got %q want %q", got.Type, want.Type)
	}
	if want.Version != got.Version {
		t.Errorf("wrong version got %q want %q", got.Version, want.Version)
	}
	if (want.PreviousEvent == nil) != (got.PreviousEvent == nil) {
		t.Errorf("linking failed got was linked: %v want was linked: %v", (got.PreviousEvent != nil), (want.PreviousEvent != nil))
	}
	if want.PreviousSequence != got.PreviousSequence {
		t.Errorf("wrong previous sequence got %d want %d", got.PreviousSequence, want.PreviousSequence)
	}
}

func TestEventstore_mapEvents(t *testing.T) {
	type fields struct {
		eventMapper map[EventType]func(*repository.Event) (Event, error)
	}
	type args struct {
		events []*repository.Event
	}
	type res struct {
		events  []Event
		wantErr bool
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		res    res
	}{
		{
			name: "no mapper",
			args: args{
				events: []*repository.Event{
					{
						Type: "no.mapper.found",
					},
				},
			},
			fields: fields{
				eventMapper: map[EventType]func(*repository.Event) (Event, error){},
			},
			res: res{
				wantErr: true,
			},
		},
		{
			name: "mapping failed",
			args: args{
				events: []*repository.Event{
					{
						Type: "test.event",
					},
				},
			},
			fields: fields{
				eventMapper: map[EventType]func(*repository.Event) (Event, error){
					"test.event": func(*repository.Event) (Event, error) {
						return nil, errors.ThrowInternal(nil, "V2-8FbQk", "test err")
					},
				},
			},
			res: res{
				wantErr: true,
			},
		},
		{
			name: "mapping succeeded",
			args: args{
				events: []*repository.Event{
					{
						Type: "test.event",
					},
				},
			},
			fields: fields{
				eventMapper: map[EventType]func(*repository.Event) (Event, error){
					"test.event": func(*repository.Event) (Event, error) {
						return &testEvent{}, nil
					},
				},
			},
			res: res{
				events: []Event{
					&testEvent{},
				},
				wantErr: false,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			es := &Eventstore{
				interceptorMutex: sync.Mutex{},
				eventMapper:      map[EventType]eventTypeInterceptors{},
			}
			for eventType, mapper := range tt.fields.eventMapper {
				es = es.RegisterFilterEventMapper(eventType, mapper)
			}
			if len(es.eventMapper) != len(tt.fields.eventMapper) {
				t.Errorf("register event mapper failed expected mapper amount: %d, got: %d", len(tt.fields.eventMapper), len(es.eventMapper))
				t.FailNow()
			}

			gotMappedEvents, err := es.mapEvents(tt.args.events)
			if (err != nil) != tt.res.wantErr {
				t.Errorf("Eventstore.mapEvents() error = %v, wantErr %v", err, tt.res.wantErr)
				return
			}
			if !reflect.DeepEqual(gotMappedEvents, tt.res.events) {
				t.Errorf("Eventstore.mapEvents() = %v, want %v", gotMappedEvents, tt.res.events)
			}
		})
	}
}
