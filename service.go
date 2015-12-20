package service

import (
	"runtime"
	"sync"
)

// State represents the state of a service.
type State int

const (
	// Starting implies the service is starting.
	Starting State = iota
	// Running implies the service is running.
	Running
	// Stopping implies stop has been requested.
	Stopping
	// Stopped implies the service is not running.
	Stopped
)

func (state State) String() string {
	switch state {
	case Starting:
		return "Starting"
	case Running:
		return "Running"
	case Stopping:
		return "Stopping"
	case Stopped:
		return "Stopped"
	default:
		return "Unknown"
	}
}

// Service represents a service running on a go-routine.
type Service interface {
	// State returns the current state of the service.
	State() State
	// Start starts the service if it isn't running.
	//
	// Eventually returns a value indicating whether the service was started.
	Start() <-chan bool
	// Stop stops the service if it is running.
	//
	// Eventually signals that the service has been stopped.
	Stop() <-chan struct{}
}

// default implementation of Service
type service struct {
	m              sync.Mutex
	state          State
	run            func(state State, cancel <-chan struct{}) error
	stateChanged   func(state State)
	onError        func(err interface{})
	stop           chan struct{}
	startListeners []chan bool
	stopListeners  []chan struct{}
}

// New creates a new Service.
func New(
	run func(state State, cancel <-chan struct{}) error,
	stateChanged func(state State),
	onError func(err interface{})) Service {
	if run == nil {
		panic("run must not be nil")
	}
	if stateChanged == nil {
		stateChanged = noOpStateChanged
	}
	if onError == nil {
		onError = noOpOnError
	}
	return &service{
		m:            sync.Mutex{},
		state:        Stopped,
		run:          run,
		stateChanged: stateChanged,
		onError:      onError,
	}
}

func noOpStateChanged(state State) {}
func noOpOnError(v interface{})    {}

func (svc *service) State() State {
	return svc.state
}

func (svc *service) Start() <-chan bool {
	svc.m.Lock()
	defer svc.m.Unlock()
	started := make(chan bool, 1)
	if svc.state == Running {
		started <- true
		close(started)
		return started
	}
	if svc.state != Starting {
		// make new so that old arrays can be collected
		svc.startListeners = make([]chan bool, 0)
		svc.stopListeners = make([]chan struct{}, 0)
	}
	svc.startListeners = append(svc.startListeners, started)
	if svc.state != Starting {
		svc.stop = make(chan struct{}, 1)
		go svc.runner()
	}
	return started
}

func (svc *service) runner() {
	svc.setState(Starting)
	firstRun := true
	var err interface{}
	for svc.state == Running || firstRun {
		runtime.Gosched()
		err = svc.runSafe()
		if err != nil {
			svc.onError(err)
			if firstRun {
				svc.signalStarted(false)
				break
			}
		} else if firstRun {
			svc.setState(Running)
			svc.signalStarted(true)
			firstRun = false
		}
	}
	svc.setState(Stopped)
	svc.signalStopped()
}

func (svc *service) Stop() <-chan struct{} {
	svc.m.Lock()
	defer svc.m.Unlock()
	stopListener := make(chan struct{}, 1)
	if svc.state == Stopped {
		stopListener <- struct{}{}
		close(stopListener)
		return stopListener
	}
	svc.stopListeners = append(svc.stopListeners, stopListener)
	if svc.state != Stopping {
		svc.setStateUnlocked(Stopping)
		svc.stop <- struct{}{}
		close(svc.stop)
	}
	return stopListener
}

func (svc *service) setState(state State) {
	svc.m.Lock()
	defer svc.m.Unlock()
	svc.setStateUnlocked(state)
}

func (svc *service) setStateUnlocked(state State) {
	svc.state = state
	svc.stateChanged(state)
}

func (svc *service) runSafe() (err interface{}) {
	defer func() {
		if r := recover(); r != nil {
			err = r
		}
		return
	}()
	return svc.run(svc.state, svc.stop)
}

func (svc *service) signalStarted(started bool) {
	svc.m.Lock()
	defer svc.m.Unlock()
	for _, startListener := range svc.startListeners {
		startListener <- started
		close(startListener)
	}
}

func (svc *service) signalStopped() {
	svc.m.Lock()
	defer svc.m.Unlock()
	for _, stopListener := range svc.stopListeners {
		stopListener <- struct{}{}
		close(stopListener)
	}
}

// StartAll starts one or more services and eventually
// returns a value indicating whether all services were started.
func StartAll(services ...Service) <-chan bool {
	started := make(chan bool, 1)
	go func() {
		allStarted := true
		if services != nil {
			var wg sync.WaitGroup
			wg.Add(len(services))
			for _, svc := range services {
				go func(svc Service) {
					if svc != nil && !<-svc.Start() {
						allStarted = false
					}
					wg.Done()
				}(svc)
			}
			wg.Wait()
		}
		started <- allStarted
		close(started)
	}()
	return started
}

// StopAll stops one or more services and eventually signals
// when all services were stopped.
func StopAll(services ...Service) <-chan struct{} {
	stopped := make(chan struct{}, 1)
	go func() {
		if services != nil {
			var wg sync.WaitGroup
			wg.Add(len(services))
			for _, svc := range services {
				go func(svc Service) {
					if svc != nil {
						<-svc.Stop()
					}
					wg.Done()
				}(svc)
			}
			wg.Wait()
		}
		stopped <- struct{}{}
		close(stopped)
	}()
	return stopped
}
