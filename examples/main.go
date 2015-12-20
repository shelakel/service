package main

import (
	"fmt"
	"log"
	"math/rand"
	"os"
	"runtime"
	"time"

	"github.com/shelakel/service"
)

func main() {
	runtime.GOMAXPROCS(1)
	count := 5
	services := make([]service.Service, 0, count)
	for i := 1; i <= count; i++ {
		name := fmt.Sprintf("Test service %d", i)
		svc := NewTestService(name)
		services = append(services, svc.Service)
	}
	fmt.Println("Starting services")
	<-service.StartAll(services...)
	<-time.After(5 * time.Second)
	fmt.Println("Stopping services")
	<-service.StopAll(services...)
}

type TestService struct {
	service.Service
	name   string
	logger *log.Logger
}

func NewTestService(name string) *TestService {
	svc := &TestService{name: name, logger: log.New(os.Stdout, "", log.Ldate|log.Ltime)}
	svc.Service = service.New(svc.run, svc.stateChanged, svc.onError)
	return svc
}

func (svc *TestService) println(v ...interface{}) {
	svc.logger.Println(append([]interface{}{svc.name}, v...)...)
}

func (svc *TestService) run(state service.State, stop <-chan struct{}) error {
	if state == service.Starting {
		// do runtime validation for permanent errors
		return nil
	}
	if state != service.Running {
		return nil
	}
	svc.println("Test")
	select {
	case <-time.After(time.Duration(rand.Int31()%5000) * time.Millisecond):
	case <-stop:
	}
	return nil
}

func (svc *TestService) stateChanged(state service.State) {
	svc.println(state)
}

func (svc *TestService) onError(err interface{}) {
	svc.println("Error:", err)
}
