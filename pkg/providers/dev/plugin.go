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

package main

import (
	"github.com/nitric-dev/membrane/pkg/plugins/document/boltdb"
	"github.com/nitric-dev/membrane/pkg/plugins/eventing/dev"
	gateway_plugin "github.com/nitric-dev/membrane/pkg/plugins/gateway/dev"
	"github.com/nitric-dev/membrane/pkg/plugins/queue/dev"
	"github.com/nitric-dev/membrane/pkg/plugins/storage/boltdb"
	"github.com/nitric-dev/membrane/pkg/sdk"
)

type DevServiceFactory struct {
}

func New() sdk.ServiceFactory {
	return &DevServiceFactory{}
}

// NewDocumentService - Returns local dev document plugin
func (p *DevServiceFactory) NewDocumentService() (sdk.DocumentService, error) {
	return boltdb_service.New()
}

// NewEventService - Returns local dev eventing plugin
func (p *DevServiceFactory) NewEventService() (sdk.EventService, error) {
	return eventing_service.New()
}

// NewGatewayService - Returns local dev Gateway plugin
func (p *DevServiceFactory) NewGatewayService() (sdk.GatewayService, error) {
	return gateway_plugin.New()
}

// NewQueueService - Returns local dev queue plugin
func (p *DevServiceFactory) NewQueueService() (sdk.QueueService, error) {
	return queue_service.New()
}

// NewStorageService - Returns local dev storage plugin
func (p *DevServiceFactory) NewStorageService() (sdk.StorageService, error) {
	return boltdb_storage_service.New()
}
