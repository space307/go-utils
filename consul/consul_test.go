package consul

import (
	"testing"

	"github.com/hashicorp/consul/testutil"
	"github.com/stretchr/testify/assert"
)

func TestFoo_bar(t *testing.T) {
	srv1, err := testutil.NewTestServerConfig(func(c *testutil.TestServerConfig) {
		c.Ports.HTTP = 8500
	})
	if err != nil {
		t.Fatal(err)
	}

	serviceName := "testService"
	client, err := NewDefaultClient(serviceName, 8080, "10m")
	assert.NoError(t, err)

	consulAgent := client.Agent()

	assert.NoError(t, client.PassingTTL(testutil.HealthPassing))

	status, info, err := consulAgent.AgentHealthServiceByID(serviceName)
	assert.NoError(t, err)
	assert.Equal(t, testutil.HealthPassing, info.Checks[0].Output)
	assert.Equal(t, testutil.HealthPassing, status)

	assert.NoError(t, client.WarningTTL(testutil.HealthWarning))

	status, info, err = consulAgent.AgentHealthServiceByID(serviceName)
	assert.NoError(t, err)
	assert.Equal(t, testutil.HealthWarning, info.Checks[0].Output)
	assert.Equal(t, testutil.HealthWarning, status)

	assert.NoError(t, client.CriticalTTL(testutil.HealthCritical))

	status, info, err = consulAgent.AgentHealthServiceByID(serviceName)
	assert.NoError(t, err)
	assert.Equal(t, testutil.HealthCritical, info.Checks[0].Output)
	assert.Equal(t, testutil.HealthCritical, status)

	assert.NoError(t, client.UpdateTTL(serviceName, testutil.HealthPassing, testutil.HealthPassing))

	status, info, err = consulAgent.AgentHealthServiceByID(serviceName)
	assert.NoError(t, err)
	assert.Equal(t, testutil.HealthPassing, info.Checks[0].Output)
	assert.Equal(t, testutil.HealthPassing, status)

	assert.NoError(t, client.Deregister())

	status, _, _ = consulAgent.AgentHealthServiceByID(serviceName)
	assert.Equal(t, testutil.HealthCritical, status)

	srv1.Stop()

	client, err = NewDefaultClient(serviceName, 8080, "10m")
	assert.NoError(t, err)
	assert.NotNil(t, client)
}
