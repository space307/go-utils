package consul

import (
	"github.com/hashicorp/consul/agent/consul"
	"github.com/hashicorp/consul/api"
)

// Client provides a client to functions of Consul API
// for one serivce
type Client struct {
	clientAPI *api.Client
	srvID     string
}

// NewDefaultClient is the most default constructor for Consul agent for one serivce.
// This function gets the local IP address and register service for a local agent with a given name.
// It ignores errors if Consul agent is unreachable.
// srvName sets for ID, Name and CheckID of service.
func NewDefaultClient(srvName string, svcPort int, checkTTL string) (*Client, error) {
	var err error

	client := &Client{srvID: srvName}

	localIP, err := consul.GetPrivateIP()
	if err != nil {
		return nil, err
	}

	client.clientAPI, err = api.NewClient(api.DefaultConfig())
	if err != nil {
		return nil, err
	}

	srvRegInfo := &api.AgentServiceRegistration{
		ID:      srvName,
		Name:    srvName,
		Address: localIP.String(),
		Port:    svcPort,
		Check: &api.AgentServiceCheck{
			CheckID: srvName,
			TTL:     checkTTL,
		},
	}

	if err = client.Register(srvRegInfo); err != nil && client.IsReachable() {
		return nil, err
	}

	return client, nil
}

// Agent returns a handle to the agent endpoints
func (c *Client) Agent() *api.Agent {
	return c.clientAPI.Agent()
}

// IsReachable check whether we can reach the agent
func (c *Client) IsReachable() bool {
	_, err := c.clientAPI.Agent().Self()

	return err == nil
}

// Register is used to register a new service with given ID
func (c *Client) Register(srvInfo *api.AgentServiceRegistration) error {
	c.srvID = srvInfo.ID
	return c.clientAPI.Agent().ServiceRegister(srvInfo)
}

// Deregister is used to deregister a service
func (c *Client) Deregister() error {
	return c.clientAPI.Agent().ServiceDeregister(c.srvID)
}

// PassingTTL is used to update the TTL of a default check
// with status 'passing'
func (c *Client) PassingTTL(output string) error {
	return c.UpdateTTL(c.srvID, api.HealthPassing, output)
}

// CriticalTTL is used to update the TTL of a default check
// with status 'critical'
func (c *Client) CriticalTTL(output string) error {
	return c.UpdateTTL(c.srvID, api.HealthCritical, output)
}

// WarningTTL is used to update the TTL of a default check
// with status 'warning'
func (c *Client) WarningTTL(output string) error {
	return c.UpdateTTL(c.srvID, api.HealthWarning, output)
}

// UpdateTTL is used to update the TTL of a check
func (c *Client) UpdateTTL(checkID, status, output string) error {
	return c.clientAPI.Agent().UpdateTTL(checkID, output, status)
}
