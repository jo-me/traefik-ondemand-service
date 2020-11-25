package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"time"


	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
)

const oneReplica = uint64(1)
const zeroReplica = uint64(0)

// Status is the service status
type Status string

const (
	// UP represents a service that is running (with at least a container running)
	UP Status = "up"
	// DOWN represents a service that is not running (with 0 container running)
	DOWN Status = "down"
	// STARTING represents a service that is starting (with at least a container starting)
	STARTING Status = "starting"
	// UNKNOWN represents a service for which the docker status is not know
	UNKNOWN Status = "unknown"
)

// Service holds all information related to a service
type Service struct {
	name      string
	timeout   uint64
	time      chan uint64
	isHandled bool
}

var services = map[string]*Service{}

func main() {
	fmt.Println("Server listening on port 10000.")
	http.HandleFunc("/", handleRequests())
	log.Fatal(http.ListenAndServe(":10000", nil))
}

func handleRequests() func(w http.ResponseWriter, r *http.Request) {
	cli, err := client.NewEnvClient()
	if err != nil {
		log.Fatal(fmt.Errorf("%+v", "Could not connect to docker API"))
	}
	return func(w http.ResponseWriter, r *http.Request) {
		serviceName, serviceTimeout, err := parseParams(r)
		if err != nil {
			fmt.Fprintf(w, "%+v", err)
		}
		service := GetOrCreateService(serviceName, serviceTimeout)
		status, err := service.HandleServiceState(cli)
		if err != nil {
			fmt.Printf("Error: %+v\n ", err)
			fmt.Fprintf(w, "%+v", err)
		}
		fmt.Fprintf(w, "%+s", status)
	}
}

func getParam(queryParams url.Values, paramName string) (string, error) {
	if queryParams[paramName] == nil {
		return "", fmt.Errorf("%s is required", paramName)
	}
	return queryParams[paramName][0], nil
}

func parseParams(r *http.Request) (string, uint64, error) {
	queryParams := r.URL.Query()

	serviceName, err := getParam(queryParams, "name")
	if err != nil {
		return "", 0, nil
	}

	timeoutString, err := getParam(queryParams, "timeout")
	if err != nil {
		return "", 0, nil
	}
	serviceTimeout, err := strconv.Atoi(timeoutString)
	if err != nil {
		return "", 0, fmt.Errorf("timeout should be an integer")
	}
	return serviceName, uint64(serviceTimeout), nil
}

// GetOrCreateService return an existing service or create one
func GetOrCreateService(name string, timeout uint64) *Service {
	if services[name] != nil {
		return services[name]
	}
	service := &Service{name, timeout, make(chan uint64), false}

	services[name] = service
	return service
}

// HandleServiceState up the service if down or set timeout for downing the service
func (service *Service) HandleServiceState(cli *client.Client) (string, error) {
	status, err := service.getStatus(cli)
	if err != nil {
		return "", err
	}
	if status == UP {
		fmt.Printf("- Service %v is up\n", service.name)
		if !service.isHandled {
			go service.stopAfterTimeout(cli)
		}
		select {
		case service.time <- service.timeout:
		default:
		}
		return "started", nil
	} else if status == STARTING {
		fmt.Printf("- Service %v is starting\n", service.name)
		if err != nil {
			return "", err
		}
		go service.stopAfterTimeout(cli)
		return "starting", nil
	} else if status == DOWN {
		fmt.Printf("- Service %v is down\n", service.name)
		service.start(cli)
		return "starting", nil
	} else {
		fmt.Printf("- Service %v status is unknown\n", service.name)
		if err != nil {
			return "", err
		}
		return service.HandleServiceState(cli)
	}
}

func (service *Service) getStatus(client *client.Client) (Status, error) {
	ctx := context.Background()
	dockerContainer, err := service.getDockerContainer(ctx, client)

	if err != nil {
		return "", err
	}

	if dockerContainer.State != "running" {
		return DOWN, nil
	}
	return UP, nil
}

func (service *Service) start(client *client.Client) {
	fmt.Printf("Starting service %s\n", service.name)
	service.isHandled = true
	service.startContainer(client)
	go service.stopAfterTimeout(client)
	service.time <- service.timeout
}

func (service *Service) stopAfterTimeout(client *client.Client) {
	service.isHandled = true
	for {
		select {
		case timeout, ok := <-service.time:
			if ok {
				time.Sleep(time.Duration(timeout) * time.Second)
			} else {
				fmt.Println("That should not happen, but we never know ;)")
			}
		default:
			fmt.Printf("Stopping service %s\n", service.name)
			service.stopContainer(client)
			return
		}
	}
}

func (service *Service) stopContainer(client *client.Client) error {
	ctx := context.Background()
	dockerContainer, err := service.getDockerContainer(ctx, client)
	if err != nil {
		return err
	}
	
	client.ContainerStop(ctx, dockerContainer.ID, nil)
	return nil

}

func (service *Service) startContainer(client *client.Client) error {
	ctx := context.Background()
	dockerContainer, err := service.getDockerContainer(ctx, client)
	if err != nil {
		return err
	}
	
	client.ContainerStart (ctx, dockerContainer.ID, types.ContainerStartOptions{})
	return nil

}

func (service *Service) getDockerContainer(ctx context.Context, client *client.Client) (*types.Container, error) {
	containers, err := client.ContainerList(context.Background(),types.ContainerListOptions{
		All:     true})

	if err != nil {
		return nil, err
	}

	/*
	for _, container := range containers {
		fmt.Printf("%s %s\n", container.Names[0], container.Image)
	}
	*/

	dockerContainer, err := findContainerByName(containers, service.name)

	if err != nil {
		return nil, err
	}


	return dockerContainer, nil
}

func findContainerByName(containers []types.Container, name string) (*types.Container, error) {
	for _, container := range containers {
		if "/" + name == container.Names[0] {
			return &container, nil
		}
	}
	return &types.Container{}, fmt.Errorf("Could not find service %s", name)
}
