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

package membrane_test

import (
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"strings"

	"github.com/nitrictech/nitric/pkg/membrane"
	"github.com/nitrictech/nitric/pkg/triggers"
	"github.com/nitrictech/nitric/pkg/worker"
	mock_worker "github.com/nitrictech/nitric/tests/mocks/worker"

	"github.com/nitrictech/nitric/pkg/plugins/document"
	"github.com/nitrictech/nitric/pkg/plugins/events"
	"github.com/nitrictech/nitric/pkg/plugins/gateway"
	"github.com/nitrictech/nitric/pkg/plugins/queue"
	"github.com/nitrictech/nitric/pkg/plugins/storage"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

type MockDocumentServer struct {
	document.UnimplementedDocumentPlugin
}

type MockeventsServer struct {
	events.UnimplementedeventsPlugin
}

type MockStorageServiceServer struct {
	storage.UnimplementedStoragePlugin
}

type MockQueueServiceServer struct {
	queue.UnimplementedQueuePlugin
}

type MockFunction struct {
	// Records the requests that its received for later inspection
	requests []*http.Request
	// Returns a fixed HTTP response
	response *http.Response
}

func (m *MockFunction) handler(rw http.ResponseWriter, req *http.Request) {

	if m.requests == nil {
		m.requests = make([]*http.Request, 0)
	}

	m.requests = append(m.requests, req)

	for key, value := range m.response.Header {
		rw.Header().Add(key, strings.Join(value, ""))
	}
	rw.WriteHeader(m.response.StatusCode)

	var rBody []byte = nil
	if m.response.Body != nil {
		rBody, _ = ioutil.ReadAll(m.response.Body)
	}

	rw.Write(rBody)
}

type MockGateway struct {
	gateway.UnimplementedGatewayPlugin
	triggers []triggers.Trigger
	// store responses for inspection
	responses []*triggers.HttpResponse
	started   bool
}

func (gw *MockGateway) Start(pool worker.WorkerPool) error {
	// Spy on the mock gateway
	gw.responses = make([]*triggers.HttpResponse, 0)

	wrkr, _ := pool.GetWorker()

	gw.started = true
	if gw.triggers != nil {
		for _, trigger := range gw.triggers {
			if s, ok := trigger.(*triggers.HttpRequest); ok {
				resp, err := wrkr.HandleHttpRequest(s)

				if err != nil {
					gw.responses = append(gw.responses, &triggers.HttpResponse{
						StatusCode: 500,
						Body:       []byte(err.Error()),
					})
				} else {
					gw.responses = append(gw.responses, resp)
				}
			} else if s, ok := trigger.(*triggers.Event); ok {
				wrkr.HandleEvent(s)
			}
		}
	}

	// Successfully end
	return nil
}

var _ = Describe("Membrane", func() {
	pool := worker.NewProcessPool(&worker.ProcessPoolOptions{})
	pool.AddWorker(mock_worker.NewMockWorker(&mock_worker.MockWorkerOptions{}))

	BeforeSuite(func() {
		os.Args = []string{}
	})

	Context("New", func() {
		Context("Tolerate Missing Services is enabled", func() {
			When("The gateway plugin is missing", func() {
				It("Should still fail to create", func() {
					m, err := membrane.New(&membrane.MembraneOptions{
						SuppressLogs:            true,
						TolerateMissingServices: true,
					})
					Expect(err).Should(HaveOccurred())
					Expect(m).To(BeNil())
				})
			})

			When("The gateway plugin is present", func() {
				mockGateway := &MockGateway{}
				mbraneOpts := membrane.MembraneOptions{
					SuppressLogs:            true,
					GatewayPlugin:           mockGateway,
					TolerateMissingServices: true,
					Pool:                    pool,
				}
				It("Should successfully create the membrane server", func() {
					m, err := membrane.New(&mbraneOpts)
					Expect(err).ShouldNot(HaveOccurred())
					Expect(m).ToNot(BeNil())
				})
			})
		})

		Context("Tolerate Missing Services is disabled", func() {
			When("Only the gateway plugin is present", func() {
				mockGateway := &MockGateway{}
				mbraneOpts := membrane.MembraneOptions{
					TolerateMissingServices: false,
					SuppressLogs:            true,
					GatewayPlugin:           mockGateway,
					Pool:                    pool,
				}
				It("Should fail to create", func() {
					m, err := membrane.New(&mbraneOpts)
					Expect(err).Should(HaveOccurred())
					Expect(m).To(BeNil())
				})
			})

			When("All plugins are present", func() {
				mockDocumentServer := &MockDocumentServer{}
				mockeventsServer := &MockeventsServer{}
				mockStorageServiceServer := &MockStorageServiceServer{}
				mockQueueServiceServer := &MockQueueServiceServer{}

				mockGateway := &MockGateway{}
				mbraneOpts := membrane.MembraneOptions{
					TolerateMissingServices: false,
					SuppressLogs:            true,
					GatewayPlugin:           mockGateway,
					DocumentPlugin:          mockDocumentServer,
					EventsPlugin:            mockeventsServer,
					StoragePlugin:           mockStorageServiceServer,
					QueuePlugin:             mockQueueServiceServer,
					Pool:                    pool,
				}

				It("Should successfully create the membrane server", func() {
					m, err := membrane.New(&mbraneOpts)
					Expect(err).ShouldNot(HaveOccurred())
					Expect(m).ToNot(BeNil())
				})
			})
		})
	})

	Context("Starting the server", func() {

		Context("That tolerates missing adapters", func() {
			When("The Gateway plugin is available and working", func() {
				mockGateway := &MockGateway{}
				os.Args = []string{}
				membrane, _ := membrane.New(&membrane.MembraneOptions{
					GatewayPlugin:           mockGateway,
					SuppressLogs:            true,
					TolerateMissingServices: true,
					Pool:                    pool,
				})

				It("Start should not error", func() {
					err := membrane.Start()
					Expect(err).ShouldNot(HaveOccurred())
				})

				It("Mock Gateways start method should have been called", func() {
					Expect(mockGateway.started).To(BeTrue())
				})
			})
		})

		When("The configured service port is already consumed", func() {
			mockGateway := &MockGateway{}
			var lis net.Listener

			membrane, _ := membrane.New(&membrane.MembraneOptions{
				GatewayPlugin:           mockGateway,
				SuppressLogs:            true,
				TolerateMissingServices: true,
				ServiceAddress:          "localhost:9005",
				Pool:                    pool,
			})

			BeforeEach(func() {
				lis, _ = net.Listen("tcp", "localhost:9005")
			})

			AfterEach(func() {
				lis.Close()
			})

			It("Should return an error", func() {
				err := membrane.Start()
				Expect(err).Should(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("Could not listen"))
			})
		})
	})

	Context("Starting the child process", func() {
		BeforeEach(func() {
			os.Args = []string{}
		})

		var mockGateway *MockGateway
		var mb *membrane.Membrane
		When("The configured command exists", func() {

			BeforeEach(func() {
				mockGateway = &MockGateway{}
				mb, _ = membrane.New(&membrane.MembraneOptions{
					ChildCommand:            []string{"echo"},
					GatewayPlugin:           mockGateway,
					ServiceAddress:          fmt.Sprintf(":%d", 9001),
					ChildTimeoutSeconds:     1,
					TolerateMissingServices: true,
					SuppressLogs:            true,
					Pool:                    pool,
				})
			})

			AfterEach(func() {
				mb.Stop()
			})

			When("There is a worker available in the pool", func() {
				BeforeEach(func() {
					go (func() {
						http.ListenAndServe(fmt.Sprintf("localhost:8081"), nil)
					})()
				})

				It("Should wait for the service to start", func() {
					err := mb.Start()
					Expect(err).ShouldNot(HaveOccurred())
				})
			})
		})

		When("The configured command does not exist", func() {
			BeforeEach(func() {
				mockGateway = &MockGateway{}

				mb, _ = membrane.New(&membrane.MembraneOptions{
					ChildAddress:            "localhost:808",
					ChildCommand:            []string{"fakecommand"},
					GatewayPlugin:           mockGateway,
					TolerateMissingServices: true,
					SuppressLogs:            true,
				})
			})

			It("Should return an error", func() {
				err := mb.Start()
				Expect(err).Should(HaveOccurred())
			})
		})
	})
})
