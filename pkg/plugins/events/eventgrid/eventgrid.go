// Copyright 2021 Nitric Pty Ltd.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package eventgrid_service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/Azure/azure-sdk-for-go/services/eventgrid/2018-01-01/eventgrid"
	"github.com/Azure/azure-sdk-for-go/services/eventgrid/2018-01-01/eventgrid/eventgridapi"
	eventgridmgmt "github.com/Azure/azure-sdk-for-go/services/eventgrid/mgmt/2020-06-01/eventgrid"
	eventgridmgmtapi "github.com/Azure/azure-sdk-for-go/services/eventgrid/mgmt/2020-06-01/eventgrid/eventgridapi"
	"github.com/Azure/go-autorest/autorest"
	"github.com/Azure/go-autorest/autorest/azure"
	"github.com/Azure/go-autorest/autorest/date"
	"github.com/nitrictech/nitric/pkg/plugins/errors"
	"github.com/nitrictech/nitric/pkg/plugins/errors/codes"
	"github.com/nitrictech/nitric/pkg/plugins/events"
	azureutils "github.com/nitrictech/nitric/pkg/providers/azure/utils"
	"github.com/nitrictech/nitric/pkg/utils"
)

type EventGridEventService struct {
	events.UnimplementedeventsPlugin
	client      eventgridapi.BaseClientAPI
	topicClient eventgridmgmtapi.TopicsClientAPI
}

func (s *EventGridEventService) ListTopics() ([]string, error) {
	newErr := errors.ErrorsWithScope(
		"EventGrid.ListTopics",
		map[string]interface{}{
			"list": "topics",
		},
	)
	//Set the topic page length
	pageLength := int32(10)

	ctx := context.Background()
	results, err := s.topicClient.ListBySubscription(ctx, "", &pageLength)

	if err != nil {
		return nil, newErr(
			codes.Internal,
			"error listing by subscription",
			err,
		)
	}

	var topics []string

	//Iterate over the topic pages adding their names to the topics slice
	for results.NotDone() {
		topicsList := results.Values()
		for _, topic := range topicsList {
			topics = append(topics, *topic.Name)
		}
		results.NextWithContext(ctx)
	}

	return topics, nil
}

func (s *EventGridEventService) getTopicEndpoint(topicName string) (string, error) {
	ctx := context.Background()
	pageLength := int32(10)
	results, err := s.topicClient.ListBySubscription(ctx, "", &pageLength)
	if err != nil {
		return "", fmt.Errorf(err.Error())
	}

	for results.NotDone() {
		topicsList := results.Values()
		for _, topic := range topicsList {
			if *topic.Name == topicName {
				return strings.TrimSuffix(strings.TrimPrefix(*topic.Endpoint, "https://"), "/api/events"), nil
			}
		}
		results.Next()
	}
	return "", fmt.Errorf("topic with provided name could not be found")
}

func (s *EventGridEventService) nitricEventsToAzureEvents(topic string, events []*events.NitricEvent) ([]eventgrid.Event, error) {
	var azureEvents []eventgrid.Event
	for _, event := range events {
		payload, err := json.Marshal(event.Payload)
		if err != nil {
			return nil, err
		}
		dataVersion := "1.0"
		azureEvents = append(azureEvents, eventgrid.Event{
			ID:          &event.ID,
			Data:        &payload,
			EventType:   &event.PayloadType,
			Subject:     &topic,
			EventTime:   &date.Time{time.Now()},
			DataVersion: &dataVersion,
		})
	}

	return azureEvents, nil
}

func (s *EventGridEventService) Publish(topic string, event *events.NitricEvent) error {
	newErr := errors.ErrorsWithScope(
		"EventGrid.Publish",
		map[string]interface{}{
			"topic": topic,
		},
	)
	ctx := context.Background()

	if len(topic) == 0 {
		return newErr(
			codes.InvalidArgument,
			"provided invalid topic",
			fmt.Errorf("non-blank topic is required"),
		)
	}
	if event == nil {
		return newErr(
			codes.InvalidArgument,
			"provided invalid event",
			fmt.Errorf("non-nil event is required"),
		)
	}

	topicHostName, err := s.getTopicEndpoint(topic)
	if err != nil {
		return err
	}
	eventToPublish, err := s.nitricEventsToAzureEvents(topicHostName, []*events.NitricEvent{event})
	if err != nil {
		return newErr(
			codes.Internal,
			"error marshalling event",
			err,
		)
	}

	result, err := s.client.PublishEvents(ctx, topicHostName, eventToPublish)
	if err != nil {
		return newErr(
			codes.Internal,
			"error publishing event",
			err,
		)
	}

	if result.StatusCode < 200 || result.StatusCode >= 300 {
		return newErr(
			codes.Internal,
			"returned non 200 status code",
			fmt.Errorf(result.Status),
		)
	}
	return nil
}

func New() (events.EventService, error) {
	subscriptionID := utils.GetEnv("AZURE_SUBSCRIPTION_ID", "")
	if len(subscriptionID) == 0 {
		return nil, fmt.Errorf("AZURE_SUBSCRIPTION_ID not configured")
	}

	//Get the event grid token, using the event grid resource endpoint
	spt, err := azureutils.GetServicePrincipalToken("https://eventgrid.azure.net")
	if err != nil {
		return nil, fmt.Errorf("error authenticating event grid client: %v", err.Error())
	}
	//Get the event grid management token using the resource management endpoint
	mgmtspt, err := azureutils.GetServicePrincipalToken(azure.PublicCloud.ResourceManagerEndpoint)
	if err != nil {
		return nil, fmt.Errorf("error authenticating event grid management client: %v", err.Error())
	}
	client := eventgrid.New()
	client.Authorizer = autorest.NewBearerAuthorizer(spt)

	topicClient := eventgridmgmt.NewTopicsClient(subscriptionID)
	topicClient.Authorizer = autorest.NewBearerAuthorizer(mgmtspt)

	return &EventGridEventService{
		client:      client,
		topicClient: topicClient,
	}, nil
}

func NewWithClient(client eventgridapi.BaseClientAPI, topicClient eventgridmgmtapi.TopicsClientAPI) (events.EventService, error) {
	return &EventGridEventService{
		client:      client,
		topicClient: topicClient,
	}, nil
}
