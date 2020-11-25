# treafik-on-demand

## Description

This is a helper service for [acouvreur's OnDemand Traefik Plugin](https://github.com/acouvreur/traefik-ondemand-plugin) and a fork of the [original OnDemand helper service](https://github.com/acouvreur/traefik-ondemand-service) adapted to work with normal docker containers instead of docker swarm.

The service provides one endpoint to request a certain container and set a timeout for how long that container should be running.
If the container is not running it will be started. When the timeout expires the container will be stopped.


## Usage

In order to use the service you should request the server according 
```
GET service_url/?name=<service_name>&timeout=<timeout>
```

`service_name`: The name of the service you want to call (and start if necessary)

`timeout`: The duration after which the service should be shut down if idle (in second)

Response:

`started`: The service is already started

`starting`: The service is starting


## Run 

To simply run the server you can use `go run main.go`.

## Deploy

To deploy this service in a container :

```
$ git clone https://github.com/jo-me/traefik-ondemand-service.git
$ docker build --tag ondemand:1.0 ./traefik-ondemand-service
$ docker run -v /var/run/docker.sock:/var/run/docker.sock ondemand:1.0
```
