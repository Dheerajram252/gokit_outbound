package base

import (
	"fmt"
	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/sd"
	"github.com/go-kit/kit/sd/consul"
	"github.com/hashicorp/consul/api"
	"github.com/pkg/errors"
	"math/rand"
	"strconv"
	"time"
)

type ServiceRegistration struct {
	ServiceName   string
	ConsulAddress string
	HTTPAddress   string
	HTTPPort      int
	Dependencies  []string
}

func Register(registration ServiceRegistration, logger log.Logger) (*api.Client, sd.Registrar, error) {
	rand.Seed(time.Now().UTC().UnixNano())

	var (
		client       consul.Client
		consulClient *api.Client
		err          error
	)
	{
		consulConfig := api.DefaultConfig()
		consulConfig.Address = registration.ConsulAddress
		consulClient, err = api.NewClient(consulConfig)
		if err != nil {
			return nil, nil, errors.WithMessage(err, "failed to initialize the consul client")
		}
		client = consul.NewClient(consulClient)
	}
	checks := api.AgentServiceChecks{}
	checks = append(checks, &api.AgentServiceCheck{
		HTTP:                           fmt.Sprintf("http://%s%s/healthcheck", registration.HTTPAddress, strconv.Itoa(registration.HTTPPort)),
		Interval:                       "1s",
		Timeout:                        "1s",
		DeregisterCriticalServiceAfter: "72h",
		Status:                         "warning",
		Notes:                          "service health check",
	},
	)
	if len(registration.Dependencies) > 0 {
		for _, dependency := range registration.Dependencies {

			checks = append(checks, &api.AgentServiceCheck{
				HTTP:                           fmt.Sprintf("http://%s%s/healthcheck", registration.HTTPAddress, strconv.Itoa(registration.HTTPPort), dependency),
				Interval:                       "1s",
				Timeout:                        "1s",
				DeregisterCriticalServiceAfter: "72h",
				Status:                         "warning",
				Notes:                          fmt.Sprintf("Service Check to monitor the health of : %s" + dependency),
			},
			)
		}
	}
	asr := api.AgentServiceRegistration{
		ID:      registration.ServiceName,
		Name:    registration.ServiceName,
		Address: registration.HTTPAddress,
		Port:    registration.HTTPPort,
		Checks:  checks,
	}

	return consulClient, consul.NewRegistrar(client, &asr, logger), err
}
