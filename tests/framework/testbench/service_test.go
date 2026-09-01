package testbench

import (
	"net/http"
	"sync"
	"testing"
)

type fakeService struct {
	name      string
	port      int
	handler   http.Handler
	stateful  bool
	partition string
}

func (s *fakeService) Name() string          { return s.name }
func (s *fakeService) Port() int             { return s.port }
func (s *fakeService) Handler() http.Handler { return s.handler }
func (s *fakeService) Stateful() bool        { return s.stateful }
func (s *fakeService) PartitionKey() string  { return s.partition }

type unpartitionedService struct{ fakeService }

func TestRegistryRejectsInvalidServices(t *testing.T) {
	handler := http.NotFoundHandler()
	tests := []struct {
		name    string
		service Service
	}{
		{name: "typed nil", service: (*fakeService)(nil)},
		{name: "blank name", service: &fakeService{port: 1, handler: handler}},
		{name: "whitespace name", service: &fakeService{name: "  ", port: 1, handler: handler}},
		{name: "invalid port", service: &fakeService{name: "service", handler: handler}},
		{name: "nil handler", service: &fakeService{name: "service", port: 1}},
		{name: "unpartitioned state", service: &unpartitionedService{fakeService{
			name: "service", port: 1, handler: handler, stateful: true,
		}}},
		{name: "unknown partition", service: &fakeService{
			name: "service", port: 1, handler: handler, stateful: true, partition: "request",
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := (&Registry{}).Register(tt.service); err == nil {
				t.Fatal("Register() accepted invalid service")
			}
		})
	}
}

func TestRegistryValidatesDuplicatesBeforeStatefulness(t *testing.T) {
	registry := &Registry{}
	handler := http.NotFoundHandler()
	if err := registry.Register(&fakeService{name: "first", port: 1, handler: handler}); err != nil {
		t.Fatal(err)
	}
	err := registry.Register(&fakeService{name: "first", port: 1, handler: handler, stateful: true})
	if err == nil || err.Error() != `testbench: services "first" and "first" both claim port 1` {
		t.Fatalf("Register() error = %v, want duplicate-port error", err)
	}
}

func TestRegistryAcceptsPartitionedState(t *testing.T) {
	registry := &Registry{}
	service := &fakeService{
		name: "analytics", port: 1, handler: http.NotFoundHandler(),
		stateful: true, partition: PartitionByBlock,
	}
	if err := registry.Register(service); err != nil {
		t.Fatal(err)
	}
}

func TestRegistrySupportsConcurrentAccess(t *testing.T) {
	registry := &Registry{}
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_ = registry.Register(&fakeService{
				name: "service-" + string(rune('a'+i)), port: i + 1,
				handler: http.NotFoundHandler(),
			})
			_ = registry.Services()
		}(i)
	}
	wg.Wait()
	if got := len(registry.Services()); got != 20 {
		t.Fatalf("registered services = %d, want 20", got)
	}
}
