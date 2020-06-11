package test

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/resgateio/resgate/server/reserr"
)

// Test model query event
func TestModelQueryEvent(t *testing.T) {
	runTest(t, func(s *Session) {
		c := s.Connect()
		subscribeToTestQueryModel(t, s, c, "q=foo&f=bar", "q=foo&f=bar")

		s.ResourceEvent("test.model", "query", json.RawMessage(`{"subject":"_EVENT_01_"}`))
		s.
			GetRequest(t).
			Equals(t, "_EVENT_01_", json.RawMessage(`{"query":"q=foo&f=bar"}`)).
			RespondSuccess(json.RawMessage(`{"events":[]}`))
	})
}

// Test query event with omitted events array
func TestModelQueryEventWithOmittedEventsToQueryRequest(t *testing.T) {
	runTest(t, func(s *Session) {
		c := s.Connect()
		subscribeToTestQueryModel(t, s, c, "q=foo&f=bar", "q=foo&f=bar")

		s.ResourceEvent("test.model", "query", json.RawMessage(`{"subject":"_EVENT_01_"}`))
		s.
			GetRequest(t).
			Equals(t, "_EVENT_01_", json.RawMessage(`{"query":"q=foo&f=bar"}`)).
			RespondSuccess(json.RawMessage(`{}`))

		c.AssertNoEvent(t, "test.model")
		s.AssertNoErrorsLogged(t)
	})
}

// Test query events on multiple queries on the same model triggers
func TestModelQueryEventOnMultipleQueries(t *testing.T) {
	runTest(t, func(s *Session) {
		c := s.Connect()
		// Subscribe with different queries to the same model
		subscribeToTestQueryModel(t, s, c, "q=foo&f=bar", "q=foo&f=bar")
		subscribeToTestQueryModel(t, s, c, "q=foo&f=baz", "q=foo&f=baz")

		// Send query event
		s.ResourceEvent("test.model", "query", json.RawMessage(`{"subject":"_EVENT_01_"}`))
		// Get query requests for the two model queries
		req1 := s.GetRequest(t).AssertSubject(t, "_EVENT_01_")
		req2 := s.GetRequest(t).AssertSubject(t, "_EVENT_01_")
		// Determine which order the query requests came and validate
		if req1.PathPayload(t, "query").(string) == "q=foo&f=bar" {
			req1.AssertPathPayload(t, "query", "q=foo&f=bar")
			req2.AssertPathPayload(t, "query", "q=foo&f=baz")
		} else {
			req1.AssertPathPayload(t, "query", "q=foo&f=baz")
			req2.AssertPathPayload(t, "query", "q=foo&f=bar")
		}
		// Send query response without events
		req1.RespondSuccess(json.RawMessage(`{}`))
		req2.RespondSuccess(json.RawMessage(`{}`))

		// Validate no events was sent to the client
		c.AssertNoEvent(t, "test.model")
		s.AssertNoErrorsLogged(t)
	})
}

// Test that query event triggers a single query request for multiple queries with
// identical normalization
func TestModelQueryEventOnSameNormalizedQueryResultsInSingleQueryRequest(t *testing.T) {
	runTest(t, func(s *Session) {
		c := s.Connect()
		// Subscribe with different queries to the same model
		subscribeToTestQueryModel(t, s, c, "q=foo&f=bar", "f=bar&q=foo")
		subscribeToTestQueryModel(t, s, c, "f=bar&q=foo&fake=1", "f=bar&q=foo")

		// Send query event
		s.ResourceEvent("test.model", "query", json.RawMessage(`{"subject":"_EVENT_01_"}`))
		// Get query requests and respond
		s.
			GetRequest(t).
			AssertPathPayload(t, "query", "f=bar&q=foo").
			RespondSuccess(json.RawMessage(`{}`))

		// Validate no events was sent to the client
		c.AssertNoEvent(t, "test.model")
		c.AssertNoNATSRequest(t, "test.model")
	})
}

// Test query event resulting in change event
func TestModelQueryEventResultingInChangeEvent(t *testing.T) {
	runTest(t, func(s *Session) {
		c := s.Connect()
		subscribeToTestQueryModel(t, s, c, "q=foo&f=bar", "q=foo&f=bar")

		// Send query event
		s.ResourceEvent("test.model", "query", json.RawMessage(`{"subject":"_EVENT_01_"}`))
		// Respond to query request with a single change event
		s.GetRequest(t).RespondSuccess(json.RawMessage(`{"events":[{"event":"change","data":{"values":{"string":"bar","int":-12}}}]}`))

		// Validate change event was sent to client
		c.GetEvent(t).Equals(t, "test.model?q=foo&f=bar.change", json.RawMessage(`{"values":{"string":"bar","int":-12}}`))
	})
}

// Test that two different model queries with the same normalized query both gets
// a client change event
func TestDifferentModelQueriesWithSameNormalizedQueryEachGetsClientChangeEvents(t *testing.T) {
	runTest(t, func(s *Session) {
		c := s.Connect()
		// Subscribe with different queries to the same model
		subscribeToTestQueryModel(t, s, c, "q=foo&f=bar", "f=bar&q=foo")
		subscribeToTestQueryModel(t, s, c, "f=bar&q=foo&fake=1", "f=bar&q=foo")

		// Send query event
		s.ResourceEvent("test.model", "query", json.RawMessage(`{"subject":"_EVENT_01_"}`))
		// Get query requests and respond
		s.GetRequest(t).RespondSuccess(json.RawMessage(`{"events":[{"event":"change","data":{"values":{"string":"bar","int":-12}}}]}`))

		// Validate client change event was sent on both query models
		evs := c.GetParallelEvents(t, 2)
		evs.GetEvent(t, "test.model?q=foo&f=bar.change").AssertData(t, json.RawMessage(`{"values":{"string":"bar","int":-12}}`))
		evs.GetEvent(t, "test.model?f=bar&q=foo&fake=1.change").AssertData(t, json.RawMessage(`{"values":{"string":"bar","int":-12}}`))
		c.AssertNoNATSRequest(t, "test.model")
	})
}

// Test query event resulting in change event
func TestModelQueryEventResponseUpdatesTheCache(t *testing.T) {
	runTest(t, func(s *Session) {
		c := s.Connect()
		subscribeToTestQueryModel(t, s, c, "q=foo&f=bar", "q=foo&f=bar")

		// Send query event
		s.ResourceEvent("test.model", "query", json.RawMessage(`{"subject":"_EVENT_01_"}`))
		// Respond to query request with a single change event
		s.GetRequest(t).RespondSuccess(json.RawMessage(`{"events":[{"event":"change","data":{"values":{"string":"bar","int":-12}}}]}`))
		// Validate change event sent to the client
		c.GetEvent(t).Equals(t, "test.model?q=foo&f=bar.change", json.RawMessage(`{"values":{"string":"bar","int":-12}}`))

		c2 := s.Connect()
		// Subscribe a second time
		creq2 := c2.Request("subscribe.test.model?q=foo&f=bar", nil)
		// Handle model access request
		s.GetRequest(t).AssertSubject(t, "access.test.model").RespondSuccess(json.RawMessage(`{"get":true}`))
		// Validate client response and validate
		creq2.GetResponse(t).AssertResult(t, json.RawMessage(`{"models":{"test.model?q=foo&f=bar":{"string":"bar","int":-12,"bool":true,"null":null}}}`))
	})
}

// Test query events on multiple queries resulting in different change events
func TestModelQueryChangeEventOnMultipleQueries(t *testing.T) {
	runTest(t, func(s *Session) {
		c := s.Connect()
		// Subscribe with different queries to the same model
		subscribeToTestQueryModel(t, s, c, "q=foo&f=bar", "q=foo&f=bar")
		subscribeToTestQueryModel(t, s, c, "q=foo&f=baz", "q=foo&f=baz")

		// Send query event
		s.ResourceEvent("test.model", "query", json.RawMessage(`{"subject":"_EVENT_01_"}`))
		// Get query requests for the two model queries
		req1 := s.GetRequest(t)
		req2 := s.GetRequest(t)
		// Determine which order the query requests came and send change response
		if req1.PathPayload(t, "query").(string) == "q=foo&f=bar" {
			req1.RespondSuccess(json.RawMessage(`{"events":[{"event":"change","data":{"values":{"string":"barbar"}}}]}`))
			req2.RespondSuccess(json.RawMessage(`{"events":[{"event":"change","data":{"values":{"string":"barbaz"}}}]}`))
		} else {
			req1.RespondSuccess(json.RawMessage(`{"events":[{"event":"change","data":{"values":{"string":"barbaz"}}}]}`))
			req2.RespondSuccess(json.RawMessage(`{"events":[{"event":"change","data":{"values":{"string":"barbar"}}}]}`))
		}

		// Validate both change events are sent to the client
		evs := c.GetParallelEvents(t, 2)
		evs.GetEvent(t, "test.model?q=foo&f=bar.change").AssertData(t, json.RawMessage(`{"values":{"string":"barbar"}}`))
		evs.GetEvent(t, "test.model?q=foo&f=baz.change").AssertData(t, json.RawMessage(`{"values":{"string":"barbaz"}}`))
	})
}

// Test query event on non-query model subscription
func TestModelQueryChangeEventOnNonQuerySubscription(t *testing.T) {
	runTest(t, func(s *Session) {
		c := s.Connect()
		// Subscribe with different queries to the same model
		subscribeToTestModel(t, s, c)

		// Send query event
		s.ResourceEvent("test.model", "query", json.RawMessage(`{"subject":"_EVENT_01_"}`))
		// Get query requests for the two model queries
		c.AssertNoNATSRequest(t, "test.model")
	})
}

// Test invalid query event on query model subscription
// These should eventually result in an error being sent to NATS.
func TestInvalidModelQueryEvent(t *testing.T) {
	tbl := []struct {
		QueryEvent []byte // Raw query event payload
	}{
		{nil},
		{[]byte(`{}`)},
		{[]byte(`{"subject":BROKEN}`)},
		{[]byte(`{"subject":42}`)},
		{[]byte(`{"subject":""}`)},
	}

	for i, l := range tbl {
		runNamedTest(t, fmt.Sprintf("#%d", i+1), func(s *Session) {
			c := s.Connect()
			subscribeToTestQueryModel(t, s, c, "q=foo&f=bar", "q=foo&f=bar")

			// Send query event
			s.ResourceEvent("test.model", "query", l.QueryEvent)

			// Assert no request is sent to NATS
			c.AssertNoNATSRequest(t, "test.model")
			s.AssertErrorsLogged(t, 1)
		})
	}
}

// Test collection query event
func TestCollectionQueryEvent(t *testing.T) {
	runTest(t, func(s *Session) {
		c := s.Connect()
		subscribeToTestQueryCollection(t, s, c, "q=foo&f=bar", "q=foo&f=bar")

		s.ResourceEvent("test.collection", "query", json.RawMessage(`{"subject":"_EVENT_01_"}`))
		s.
			GetRequest(t).
			Equals(t, "_EVENT_01_", json.RawMessage(`{"query":"q=foo&f=bar"}`)).
			RespondSuccess(json.RawMessage(`{"events":[]}`))
	})
}

// Test query event with omitted events array
func TestCollectionQueryEventWithOmittedEventsToQueryRequest(t *testing.T) {
	runTest(t, func(s *Session) {
		c := s.Connect()
		subscribeToTestQueryCollection(t, s, c, "q=foo&f=bar", "q=foo&f=bar")

		s.ResourceEvent("test.collection", "query", json.RawMessage(`{"subject":"_EVENT_01_"}`))
		s.
			GetRequest(t).
			Equals(t, "_EVENT_01_", json.RawMessage(`{"query":"q=foo&f=bar"}`)).
			RespondSuccess(json.RawMessage(`{}`))

		c.AssertNoEvent(t, "test.collection")
		s.AssertNoErrorsLogged(t)
	})
}

// Test query events on multiple queries on the same collection triggers
func TestCollectionQueryEventOnMultipleQueries(t *testing.T) {
	runTest(t, func(s *Session) {
		c := s.Connect()
		// Subscribe with different queries to the same collection
		subscribeToTestQueryCollection(t, s, c, "q=foo&f=bar", "q=foo&f=bar")
		subscribeToTestQueryCollection(t, s, c, "q=foo&f=baz", "q=foo&f=baz")

		// Send query event
		s.ResourceEvent("test.collection", "query", json.RawMessage(`{"subject":"_EVENT_01_"}`))
		// Get query requests for the two collection queries
		req1 := s.GetRequest(t).AssertSubject(t, "_EVENT_01_")
		req2 := s.GetRequest(t).AssertSubject(t, "_EVENT_01_")
		// Determine which order the query requests came and validate
		if req1.PathPayload(t, "query").(string) == "q=foo&f=bar" {
			req1.AssertPathPayload(t, "query", "q=foo&f=bar")
			req2.AssertPathPayload(t, "query", "q=foo&f=baz")
		} else {
			req1.AssertPathPayload(t, "query", "q=foo&f=baz")
			req2.AssertPathPayload(t, "query", "q=foo&f=bar")
		}
		// Send query response without events
		req1.RespondSuccess(json.RawMessage(`{}`))
		req2.RespondSuccess(json.RawMessage(`{}`))

		// Validate no events was sent to the client
		c.AssertNoEvent(t, "test.collection")
		s.AssertNoErrorsLogged(t)
	})
}

// Test that query event triggers a single query request for multiple queries with
// identical normalization
func TestCollectionQueryEventOnSameNormalizedQueryResultsInSingleQueryRequest(t *testing.T) {
	runTest(t, func(s *Session) {
		c := s.Connect()
		// Subscribe with different queries to the same collection
		subscribeToTestQueryCollection(t, s, c, "q=foo&f=bar", "f=bar&q=foo")
		subscribeToTestQueryCollection(t, s, c, "f=bar&q=foo&fake=1", "f=bar&q=foo")

		// Send query event
		s.ResourceEvent("test.collection", "query", json.RawMessage(`{"subject":"_EVENT_01_"}`))
		// Get query requests and respond
		s.
			GetRequest(t).
			AssertPathPayload(t, "query", "f=bar&q=foo").
			RespondSuccess(json.RawMessage(`{}`))

		// Validate no events was sent to the client
		c.AssertNoEvent(t, "test.collection")
		c.AssertNoNATSRequest(t, "test.collection")
	})
}

// Test query event resulting in change event
func TestCollectionQueryEventResultingInAddRemoveEvent(t *testing.T) {
	runTest(t, func(s *Session) {
		eventAdd := `{"idx":1,"value":"bar"}`
		eventRemove := `{"idx":4}`
		c := s.Connect()
		subscribeToTestQueryCollection(t, s, c, "q=foo&f=bar", "q=foo&f=bar")

		// Send query event
		s.ResourceEvent("test.collection", "query", json.RawMessage(`{"subject":"_EVENT_01_"}`))
		// Respond to query request with a single change event
		s.GetRequest(t).RespondSuccess(json.RawMessage(`{"events":[{"event":"add","data":` + eventAdd + `},{"event":"remove","data":` + eventRemove + `}]}`))

		// Validate change event was sent to client
		c.GetEvent(t).Equals(t, "test.collection?q=foo&f=bar.add", json.RawMessage(eventAdd))
		c.GetEvent(t).Equals(t, "test.collection?q=foo&f=bar.remove", json.RawMessage(eventRemove))
	})
}

// Test that two different collection queries with the same normalized query both gets
// a client change event
func TestDifferentCollectionQueriesWithSameNormalizedQueryEachGetsClientChangeEvents(t *testing.T) {
	runTest(t, func(s *Session) {
		c := s.Connect()
		eventAdd := `{"idx":1,"value":"bar"}`
		// Subscribe with different queries to the same collection
		subscribeToTestQueryCollection(t, s, c, "q=foo&f=bar", "f=bar&q=foo")
		subscribeToTestQueryCollection(t, s, c, "f=bar&q=foo&fake=1", "f=bar&q=foo")

		// Send query event
		s.ResourceEvent("test.collection", "query", json.RawMessage(`{"subject":"_EVENT_01_"}`))
		// Get query requests and respond
		s.GetRequest(t).RespondSuccess(json.RawMessage(`{"events":[{"event":"add","data":` + eventAdd + `}]}`))

		// Validate client change event was sent on both query collections
		evs := c.GetParallelEvents(t, 2)
		evs.GetEvent(t, "test.collection?q=foo&f=bar.add").AssertData(t, json.RawMessage(eventAdd))
		evs.GetEvent(t, "test.collection?f=bar&q=foo&fake=1.add").AssertData(t, json.RawMessage(eventAdd))
		c.AssertNoNATSRequest(t, "test.collection")
	})
}

// Test query event resulting in change event
func TestCollectionQueryEventResponseUpdatesTheCache(t *testing.T) {
	runTest(t, func(s *Session) {
		eventAdd := `{"idx":1,"value":"bar"}`
		eventRemove := `{"idx":4}`
		c := s.Connect()
		subscribeToTestQueryCollection(t, s, c, "q=foo&f=bar", "q=foo&f=bar")

		// Send query event
		s.ResourceEvent("test.collection", "query", json.RawMessage(`{"subject":"_EVENT_01_"}`))
		// Respond to query request with a single change event
		s.GetRequest(t).RespondSuccess(json.RawMessage(`{"events":[{"event":"add","data":` + eventAdd + `},{"event":"remove","data":` + eventRemove + `}]}`))
		// Validate change event sent to the client
		c.GetEvent(t).Equals(t, "test.collection?q=foo&f=bar.add", json.RawMessage(json.RawMessage(eventAdd)))
		c.GetEvent(t).Equals(t, "test.collection?q=foo&f=bar.remove", json.RawMessage(json.RawMessage(eventRemove)))

		c2 := s.Connect()
		// Subscribe a second time
		creq2 := c2.Request("subscribe.test.collection?q=foo&f=bar", nil)
		// Handle collection get and access request
		mreqs2 := s.GetParallelRequests(t, 1)
		mreqs2.GetRequest(t, "access.test.collection").RespondSuccess(json.RawMessage(`{"get":true}`))
		// Validate client response and validate
		creq2.GetResponse(t).AssertResult(t, json.RawMessage(`{"collections":{"test.collection?q=foo&f=bar":["foo","bar",42,true]}}`))
	})
}

// Test query events on multiple queries resulting in different change events
func TestCollectionQueryChangeEventOnMultipleQueries(t *testing.T) {
	runTest(t, func(s *Session) {
		eventAddBar := `{"idx":1,"value":"bar"}`
		eventAddBaz := `{"idx":1,"value":"baz"}`

		c := s.Connect()
		// Subscribe with different queries to the same collection
		subscribeToTestQueryCollection(t, s, c, "q=foo&f=bar", "q=foo&f=bar")
		subscribeToTestQueryCollection(t, s, c, "q=foo&f=baz", "q=foo&f=baz")

		// Send query event
		s.ResourceEvent("test.collection", "query", json.RawMessage(`{"subject":"_EVENT_01_"}`))
		// Get query requests for the two collection queries
		req1 := s.GetRequest(t)
		req2 := s.GetRequest(t)
		// Determine which order the query requests came and send change response
		if req1.PathPayload(t, "query").(string) == "q=foo&f=bar" {
			req1.RespondSuccess(json.RawMessage(`{"events":[{"event":"add","data":` + eventAddBar + `}]}`))
			req2.RespondSuccess(json.RawMessage(`{"events":[{"event":"add","data":` + eventAddBaz + `}]}`))
		} else {
			req1.RespondSuccess(json.RawMessage(`{"events":[{"event":"add","data":` + eventAddBaz + `}]}`))
			req2.RespondSuccess(json.RawMessage(`{"events":[{"event":"add","data":` + eventAddBar + `}]}`))
		}

		// Validate both change events are sent to the client
		evs := c.GetParallelEvents(t, 2)
		evs.GetEvent(t, "test.collection?q=foo&f=bar.add").AssertData(t, json.RawMessage(eventAddBar))
		evs.GetEvent(t, "test.collection?q=foo&f=baz.add").AssertData(t, json.RawMessage(eventAddBaz))
	})
}

// Test that query events locks the event queue until all its query responses are handled
func TestQueryEventQueuesOtherEventsUntilHandled(t *testing.T) {
	runTest(t, func(s *Session) {
		c := s.Connect()
		subscribeToTestModel(t, s, c)
		subscribeToTestQueryModel(t, s, c, "q=foo&f=bar", "q=foo&f=bar")

		s.ResourceEvent("test.model", "query", json.RawMessage(`{"subject":"_EVENT_01_"}`))
		s.ResourceEvent("test.model", "change", json.RawMessage(`{"values":{"string":"bar","int":-12}}`))
		s.
			GetRequest(t).
			Equals(t, "_EVENT_01_", json.RawMessage(`{"query":"q=foo&f=bar"}`)).
			RespondSuccess(json.RawMessage(`{"events":[{"event":"change","data":{"values":{"string":"baz","int":-13}}}]}`))

		// Validate query change event was sent to client first
		c.GetEvent(t).Equals(t, "test.model?q=foo&f=bar.change", json.RawMessage(`{"values":{"string":"baz","int":-13}}`))
		// Validate model change event was sent afterwards
		c.GetEvent(t).Equals(t, "test.model.change", json.RawMessage(`{"values":{"string":"bar","int":-12}}`))
	})
}

// Test that a query event with multiple query requests doesn't deadlock if an regular event gets queued
func TestQueryEventWithMultipleQueryRequestsQueuesOtherEventsUntilHandled(t *testing.T) {
	runTest(t, func(s *Session) {
		c := s.Connect()
		subscribeToTestModel(t, s, c)
		// Subscribe with different queries to the same model
		subscribeToTestQueryModel(t, s, c, "q=foo&f=bar", "q=foo&f=bar")
		subscribeToTestQueryModel(t, s, c, "q=foo&f=baz", "q=foo&f=baz")

		// Send query event
		s.ResourceEvent("test.model", "query", json.RawMessage(`{"subject":"_EVENT_01_"}`))
		// Send regular event
		s.ResourceEvent("test.model", "change", json.RawMessage(`{"values":{"string":"bar","int":-12}}`))

		// Get query requests for the two model queries
		req1 := s.GetRequest(t)
		req2 := s.GetRequest(t)
		// Determine which order the query requests came and send change response
		if req1.PathPayload(t, "query").(string) == "q=foo&f=bar" {
			req1.RespondSuccess(json.RawMessage(`{"events":[{"event":"change","data":{"values":{"string":"barbar"}}}]}`))
			req2.RespondSuccess(json.RawMessage(`{"events":[{"event":"change","data":{"values":{"string":"barbaz"}}}]}`))
		} else {
			req1.RespondSuccess(json.RawMessage(`{"events":[{"event":"change","data":{"values":{"string":"barbaz"}}}]}`))
			req2.RespondSuccess(json.RawMessage(`{"events":[{"event":"change","data":{"values":{"string":"barbar"}}}]}`))
		}

		// Validate both query change event were sent to client first
		evs := c.GetParallelEvents(t, 2)
		evs.GetEvent(t, "test.model?q=foo&f=bar.change").AssertData(t, json.RawMessage(`{"values":{"string":"barbar"}}`))
		evs.GetEvent(t, "test.model?q=foo&f=baz.change").AssertData(t, json.RawMessage(`{"values":{"string":"barbaz"}}`))
		// Validate model change event was sent afterwards
		c.GetEvent(t).Equals(t, "test.model.change", json.RawMessage(`{"values":{"string":"bar","int":-12}}`))
	})
}

// Test query event discarded if sent prior to get response
func TestQueryEventDiscardedWhenPrecedingGetResponse(t *testing.T) {
	runTest(t, func(s *Session) {
		c := s.Connect()
		model := resourceData("test.model")

		// Send subscribe request
		creq := c.Request("subscribe.test.model?foo=bar", nil)

		// Handle model get and access request
		mreqs := s.GetParallelRequests(t, 2)

		// Send query event
		s.ResourceEvent("test.model", "query", json.RawMessage(`{"subject":"_EVENT_01_"}`))

		// Send get and access response
		mreqs.GetRequest(t, "get.test.model").RespondSuccess(json.RawMessage(`{"model":` + model + `,"query":"foo=baz"}`))
		mreqs.GetRequest(t, "access.test.model").RespondSuccess(json.RawMessage(`{"get":true}`))

		// Validate client response and validate
		creq.GetResponse(t).AssertResult(t, json.RawMessage(`{"models":{"test.model?foo=bar":`+model+`}}`))

		c.AssertNoEvent(t, "test.model")
		s.AssertNoErrorsLogged(t)
	})
}

func TestQueryEvent_ModelResponse_CausesChangeEvent(t *testing.T) {
	runTest(t, func(s *Session) {
		c := s.Connect()
		subscribeToTestQueryModel(t, s, c, "q=foo&f=bar", "q=foo&f=bar")

		// Send query event
		s.ResourceEvent("test.model", "query", json.RawMessage(`{"subject":"_EVENT_01_"}`))
		// Respond to query request with a model
		s.GetRequest(t).RespondSuccess(json.RawMessage(`{"model":{"string":"bar","int":-12,"bool":true}}`))
		// Validate change event was sent to client
		c.GetEvent(t).Equals(t, "test.model?q=foo&f=bar.change", json.RawMessage(`{"values":{"string":"bar","int":-12,"null":{"action":"delete"}}}`))
	})
}

func TestQueryEvent_CollectionResponse_CausesAddRemoveEvents(t *testing.T) {
	runTest(t, func(s *Session) {
		c := s.Connect()
		subscribeToTestQueryCollection(t, s, c, "q=foo&f=bar", "q=foo&f=bar")

		// Send query event
		s.ResourceEvent("test.collection", "query", json.RawMessage(`{"subject":"_EVENT_01_"}`))
		// Respond to query request with a collection
		s.GetRequest(t).RespondSuccess(json.RawMessage(`{"collection":["foo","bar",42,true]}`))

		// Validate change event was sent to client
		c.GetEvent(t).Equals(t, "test.collection?q=foo&f=bar.remove", json.RawMessage(`{"idx":3}`))
		c.GetEvent(t).Equals(t, "test.collection?q=foo&f=bar.add", json.RawMessage(`{"idx":1,"value":"bar"}`))
	})
}

func TestQueryEvent_CollectionResponseOnModel_CausesErrorLog(t *testing.T) {
	runTest(t, func(s *Session) {
		c := s.Connect()
		subscribeToTestQueryModel(t, s, c, "q=foo&f=bar", "q=foo&f=bar")

		// Send query event
		s.ResourceEvent("test.model", "query", json.RawMessage(`{"subject":"_EVENT_01_"}`))
		// Respond to query request with a collection
		s.GetRequest(t).RespondSuccess(json.RawMessage(`{"collection":["foo","bar",42,true]}`))

		// Validate no events was sent to client and an error was logged
		c.AssertNoEvent(t, "test.model")
		s.AssertErrorsLogged(t, 1)
	})
}

func TestQueryEvent_ModelResponseOnCollection_CausesErrorLog(t *testing.T) {
	runTest(t, func(s *Session) {
		c := s.Connect()
		subscribeToTestQueryCollection(t, s, c, "q=foo&f=bar", "q=foo&f=bar")

		// Send query event
		s.ResourceEvent("test.collection", "query", json.RawMessage(`{"subject":"_EVENT_01_"}`))
		// Respond to query request with a model
		s.GetRequest(t).RespondSuccess(json.RawMessage(`{"model":{"string":"bar","int":-12,"bool":true}}`))

		// Validate no events was sent to client and an error was logged
		c.AssertNoEvent(t, "test.collection")
		s.AssertErrorsLogged(t, 1)
	})
}

func TestQueryEvent_InvalidResponseOnModelResource_CausesErrorLog(t *testing.T) {
	tbl := []struct {
		InvalidQueryResponse string // Raw query event payload
	}{
		{`{"events":"foo"}`},
		{`{"model":[]}`},
		{`{"collection":[]}`},
		{`{"model":{},"events":[]}`},
		{`{"model":{"string":"bar"},"events":[]}`},
		{`{"model":{},"events":[{"event":"change","data":{"values":{"string":"bar","int":-12}}}]}`},
		{`{"model":{"string":"bar"},"events":[{"event":"change","data":{"values":{"string":"bar","int":-12}}}]}`},
	}

	for i, l := range tbl {
		runNamedTest(t, fmt.Sprintf("#%d", i+1), func(s *Session) {
			c := s.Connect()
			subscribeToTestQueryModel(t, s, c, "q=foo&f=bar", "q=foo&f=bar")

			// Send query event
			s.ResourceEvent("test.model", "query", json.RawMessage(`{"subject":"_EVENT_01_"}`))
			// Respond to query request invalid response
			s.GetRequest(t).RespondSuccess(json.RawMessage(l.InvalidQueryResponse))

			// Validate no events was sent to client and an error was logged
			c.AssertNoEvent(t, "test.model")
			s.AssertErrorsLogged(t, 1)
		})
	}
}

func TestQueryEvent_InvalidResponseOnCollectionResource_CausesErrorLog(t *testing.T) {
	tbl := []struct {
		InvalidQueryResponse string // Raw query event payload
	}{
		{`{"events":"foo"}`},
		{`{"collection":{}}`},
		{`{"model":{}}`},
		{`{"collection":[],"events":[]}`},
		{`{"collection":["foo","bar"],"events":[]}`},
		{`{"collection":[],"events":[{"event":"add","data":{"idx":1,"value":"bar"}}]}`},
		{`{"collection":["foo","bar"],"events":[{"event":"add","data":{"idx":1,"value":"bar"}}]}`},
	}

	for i, l := range tbl {
		runNamedTest(t, fmt.Sprintf("#%d", i+1), func(s *Session) {
			c := s.Connect()
			subscribeToTestQueryCollection(t, s, c, "q=foo&f=bar", "q=foo&f=bar")

			// Send query event
			s.ResourceEvent("test.collection", "query", json.RawMessage(`{"subject":"_EVENT_01_"}`))
			// Respond to query request invalid response
			s.GetRequest(t).RespondSuccess(json.RawMessage(l.InvalidQueryResponse))

			// Validate no events was sent to client and an error was logged
			c.AssertNoEvent(t, "test.collection")
			s.AssertErrorsLogged(t, 1)
		})
	}
}

func TestQueryEvent_NotFoundResponseOnModel_GeneratesDeleteEvent(t *testing.T) {
	runTest(t, func(s *Session) {
		c := s.Connect()
		subscribeToTestQueryModel(t, s, c, "q=foo&f=bar", "q=foo&f=bar")
		// Send query event
		s.ResourceEvent("test.model", "query", json.RawMessage(`{"subject":"_EVENT_01_"}`))
		// Respond to query request with an error
		s.GetRequest(t).RespondError(reserr.ErrNotFound)
		// Validate delete event is sent to client
		c.GetEvent(t).AssertEventName(t, "test.model?q=foo&f=bar.delete").AssertData(t, nil)
		// Validate subsequent query events does not send request
		s.ResourceEvent("test.model", "query", json.RawMessage(`{"subject":"_EVENT_02_"}`))
		c.AssertNoNATSRequest(t, "test.model")
	})
}

func TestQueryEvent_NotFoundResponseOnCollection_GeneratesDeleteEvent(t *testing.T) {
	runTest(t, func(s *Session) {
		c := s.Connect()
		subscribeToTestQueryCollection(t, s, c, "q=foo&f=bar", "q=foo&f=bar")

		// Send query event
		s.ResourceEvent("test.collection", "query", json.RawMessage(`{"subject":"_EVENT_01_"}`))
		// Respond to query request with an error
		s.GetRequest(t).RespondError(reserr.ErrNotFound)
		// Validate delete event is sent to client
		c.GetEvent(t).AssertEventName(t, "test.collection?q=foo&f=bar.delete").AssertData(t, nil)
		// Validate subsequent query events does not send request
		s.ResourceEvent("test.collection", "query", json.RawMessage(`{"subject":"_EVENT_02_"}`))
		c.AssertNoNATSRequest(t, "test.collection")
	})
}

func TestQueryEvent_InternalErrorResponseOnModel_LogsError(t *testing.T) {
	runTest(t, func(s *Session) {
		c := s.Connect()
		subscribeToTestQueryModel(t, s, c, "q=foo&f=bar", "q=foo&f=bar")

		// Send query event
		s.ResourceEvent("test.model", "query", json.RawMessage(`{"subject":"_EVENT_01_"}`))
		// Respond to query request with an error
		s.GetRequest(t).RespondError(reserr.ErrInternalError)
		// Validate delete event is sent to client
		c.AssertNoEvent(t, "test.model")
		// Assert error is logged
		s.AssertErrorsLogged(t, 1)
		// Validate subsequent query events does send request
		s.ResourceEvent("test.model", "query", json.RawMessage(`{"subject":"_EVENT_02_"}`))
		s.GetRequest(t).RespondSuccess(json.RawMessage(`{"events":[]}`))
	})
}

func TestQueryEvent_InternalErrorResponseOnCollection_LogsError(t *testing.T) {
	runTest(t, func(s *Session) {
		c := s.Connect()
		subscribeToTestQueryCollection(t, s, c, "q=foo&f=bar", "q=foo&f=bar")

		// Send query event
		s.ResourceEvent("test.collection", "query", json.RawMessage(`{"subject":"_EVENT_01_"}`))
		// Respond to query request with an error
		s.GetRequest(t).RespondError(reserr.ErrInternalError)
		// Validate delete event is sent to client
		c.AssertNoEvent(t, "test.collection")
		// Assert error is logged
		s.AssertErrorsLogged(t, 1)
		// Validate subsequent query events does send request
		s.ResourceEvent("test.collection", "query", json.RawMessage(`{"subject":"_EVENT_02_"}`))
		s.GetRequest(t).RespondSuccess(json.RawMessage(`{"events":[]}`))
	})
}

func TestQueryEvent_DeleteEventOnModel_DeletesFromCache(t *testing.T) {
	runTest(t, func(s *Session) {
		c := s.Connect()
		subscribeToTestQueryModel(t, s, c, "q=foo&f=bar", "q=foo&f=bar")
		// Send query event
		s.ResourceEvent("test.model", "query", json.RawMessage(`{"subject":"_EVENT_01_"}`))
		// Respond to query request with an error
		s.GetRequest(t).RespondSuccess(json.RawMessage(`{"events":[{"event":"delete"},{"event":"change","data":{"values":{"string":"bar","int":-12}}}]}`))
		// Validate only delete event is sent to client
		c.GetEvent(t).AssertEventName(t, "test.model?q=foo&f=bar.delete").AssertData(t, nil)
		c.AssertNoEvent(t, "test.model")
		// Validate subsequent query events does not send request
		s.ResourceEvent("test.model", "query", json.RawMessage(`{"subject":"_EVENT_02_"}`))
		c.AssertNoNATSRequest(t, "test.model")
	})
}

func TestQueryEvent_DeleteEventOnCollection_DeletesFromCache(t *testing.T) {
	runTest(t, func(s *Session) {
		c := s.Connect()
		subscribeToTestQueryCollection(t, s, c, "q=foo&f=bar", "q=foo&f=bar")
		// Send query event
		s.ResourceEvent("test.collection", "query", json.RawMessage(`{"subject":"_EVENT_01_"}`))
		// Respond to query request with an error
		s.GetRequest(t).RespondSuccess(json.RawMessage(`{"events":[{"event":"delete"},{"event":"add","data":{"idx":1,"value":"bar"}}]}`))
		// Validate only delete event is sent to client
		c.GetEvent(t).AssertEventName(t, "test.collection?q=foo&f=bar.delete").AssertData(t, nil)
		c.AssertNoEvent(t, "test.collection")
		// Validate subsequent query events does not send request
		s.ResourceEvent("test.collection", "query", json.RawMessage(`{"subject":"_EVENT_02_"}`))
		c.AssertNoNATSRequest(t, "test.collection")
	})
}

func TestQueryEvent_MultipleClientsWithDifferentQueries_SendsMultipleQueryRequest(t *testing.T) {
	runTest(t, func(s *Session) {
		c1 := s.Connect()
		c2 := s.Connect()
		subscribeToTestQueryCollection(t, s, c1, "q=foo&f=bar", "q=foo&f=bar")
		subscribeToTestQueryCollection(t, s, c2, "q=zoo&f=baz", "q=zoo&f=baz")
		// Send query event
		s.ResourceEvent("test.collection", "query", json.RawMessage(`{"subject":"_EVENT_01_"}`))
		// Respond to query request with an error
		mreqs := s.GetParallelRequests(t, 2)

		// Ensure order
		if mreqs[0].PathPayload(t, "query").(string) == "q=zoo&f=baz" {
			temp := mreqs[0]
			mreqs[0] = mreqs[1]
			mreqs[1] = temp
		}
		// Validate query requests
		mreqs[0].AssertSubject(t, "_EVENT_01_").AssertPathPayload(t, "query", "q=foo&f=bar")
		mreqs[1].AssertSubject(t, "_EVENT_01_").AssertPathPayload(t, "query", "q=zoo&f=baz")
		// Send query response
		mreqs[0].RespondSuccess(json.RawMessage(`{"events":[]}`))
		mreqs[1].RespondSuccess(json.RawMessage(`{"events":[]}`))
		// Validate no events
		c1.AssertNoEvent(t, "test.collection")
		c2.AssertNoEvent(t, "test.collection")
	})
}
