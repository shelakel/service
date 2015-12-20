# Package service

An abstraction for Go services to run in a single process with graceful shutdown.

Ideal for message queue processors, schedulers etc.

## Usage

See examples.

```go
import (
  "fmt"
  "log"
  "time"

  "github.com/shelakel/service"  
)

func main() {
  svc := NewMyService()
  if !<-svc.Start() {
    fmt.Println("My Service failed to start.")
  }
  <-time.After(12 * time.Second)
  <-svc.Stop() // wait for the service to stop gracefully
}

type MyService struct {
  name string
  logger *log.Logger
}

func NewMyService() *MyService {
  svc := &MyService{name: "My Service", logger: log.New(os.Stdout, "", log.Ldate|log.Ltime)}
  svc.Service = service.New(svc.run, svc.stateChanged, svc.onError)
  return svc
}

func(svc *MyService) run(state service.State, stop <-chan struct{}) error {
  if state == service.Starting {
    // if a panic happens or an error is returned when the service is starting,
    // the service will be stopped and "false" will be returned via the Start()
    // channel. Ideal for runtime validation of dependencies.
    return nil
  }
  if state != service.Running {
    return nil // do nothing
  }  
  svc.logger.Println(svc.name, state) // do something
  select { // delay for 5 seconds, break on stop requested
    case <-stop: return nil
    case time.After(5 * time.Second): return nil
  }
  return nil
}

func (svc *MyService) stateChanged(state service.State) {
  svc.logger.Println(svc.name, state)
}

func (svc *MyService) onError(err interface{}) {
  svc.logger.Println(svc.name, "Error:", err)
}
```

## Issues

Please open a ticket.

## License

See LICENSE (MIT).

## TODO

Some better examples.
